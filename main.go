package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gocv.io/x/gocv"
)

const (
	// name is a program name
	name = "restricted-zone-notifier"
	// topic is MQTT topic
	topic = "machine/zone"
	// alert contains alert message which will get displayed in main window
	alert = "HUMAN IN RESTRICTED ZONE: PAUSE THE MACHINE!"
)

var (
	// deviceID is camera device ID
	deviceID int
	// input is path to image or video file
	input string
	// model is path to .bin file of pedestrian detection model
	model string
	// modelConfig is path to .xml file of pedestrian detection model configuration
	modelConfig string
	// modelConfidence is confidence threshold for pedestrian detection model
	modelConfidence float64
	// pointX is X coordinate of the top left point of restricted zone on camera feed
	pointX int
	// pointY coordinate of the top left point of restricted zone on camera feed
	pointY int
	// width is width of the restricted zone in pixels
	width int
	// height is height of the restricted zone in pixels
	height int
	// backend is inference backend
	backend int
	// target is inference target
	target int
	// publish is a flag which instructs the program to publish data analytics
	publish bool
	// rate is number of seconds between analytics are collected and sent to a remote server
	rate int
	// delay is video playback delay
	delay float64
)

func init() {
	flag.IntVar(&deviceID, "device", -1, "Camera device ID")
	flag.StringVar(&input, "input", "", "Path to image or video file")
	flag.StringVar(&model, "model", "", "Path to .bin file of pedestrian detection model")
	flag.StringVar(&modelConfig, "model-config", "", "Path to .xml file of model configuration")
	flag.Float64Var(&modelConfidence, "model-confidence", 0.5, "Confidence threshold for pedestrian detection")
	flag.IntVar(&target, "target", 0, "Target device. 0: CPU, 1: OpenCL, 2: OpenCL half precision, 3: VPU")
	flag.IntVar(&backend, "backend", 0, "Inference backend. 0: Auto, 1: Halide language, 2: Intel DL Inference Engine")
	flag.IntVar(&pointX, "x", 0, "X coordinate of the top left point of restricted zone in camera feed")
	flag.IntVar(&pointY, "y", 0, "Y coordinate of the top left point of restricted zone in camera feed")
	flag.IntVar(&width, "width", 0, "Width of the restricted zone in pixels")
	flag.IntVar(&height, "height", 0, "Height of the restricted zone in pixels")
	flag.BoolVar(&publish, "publish", false, "Publish data analytics to a remote server")
	flag.IntVar(&rate, "rate", 1, "Number of seconds between analytics are sent to a remote server")
	flag.Float64Var(&delay, "delay", 5.0, "Video playback delay")
}

// Perf stores inference engine performance info
type Perf struct {
	// Net stores pedestrian detector performance info
	Net float64
}

// String implements fmt.Stringer interface for Perf
func (p *Perf) String() string {
	return fmt.Sprintf("Inference time: %.2f ms", p.Net)
}

// Result is computation result of the monitor returned to main goroutine
type Result struct {
	// Alert is used to raise an alert based on the presence of pedestrians in restricted area
	Alert bool
	// Perf is inference engine performance
	Perf *Perf
}

// String implements fmt.Stringer interface for Result
func (r *Result) String() string {
	return fmt.Sprintf("Safe %v", !r.Alert)
}

// ToMQTTMessage turns result into MQTT message which can be published to MQTT broker
func (r *Result) ToMQTTMessage() string {
	return fmt.Sprintf("{\"Safe\" : %v}", !r.Alert)
}

// getPerformanceInfo queries the Inference Engine performance info and returns it as string
func getPerformanceInfo(net *gocv.Net) *Perf {
	freq := gocv.GetTickFrequency() / 1000

	perf := net.GetPerfProfile() / freq

	return &Perf{
		Net: perf,
	}
}

// messageRunner reads data published to pubChan with rate frequency and sends them to remote analytics server
// doneChan is used to receive a signal from the main goroutine to notify the routine to stop and return
func messageRunner(doneChan <-chan struct{}, pubChan <-chan *Result, c *MQTTClient, topic string, rate int) error {
	ticker := time.NewTicker(time.Duration(rate) * time.Second)

	for {
		select {
		case <-ticker.C:
			result := <-pubChan
			_, err := c.Publish(topic, result.ToMQTTMessage())
			// TODO: decide whether to return with error and stop program;
			// For now we just signal there was an error and carry on
			if err != nil {
				fmt.Printf("Error publishing message to %s: %v", topic, err)
			}
		case <-pubChan:
			// we discard messages in between ticker times
		case <-doneChan:
			fmt.Printf("Stopping messageRunner: received stop sginal\n")
			return nil
		}
	}

	return nil
}

// detectMotion detects pedstrian motion in restricted zone area and returns bool based on the result of detection
func detectMotion(img *gocv.Mat, persons []image.Rectangle, area *image.Rectangle) bool {
	for i := range persons {
		if !persons[i].In(image.Rect(0, 0, img.Cols(), img.Rows())) {
			continue
		}

		if persons[i].In(*area) {
			return true
		}
	}

	return false
}

// detectPersons detects pedestrians in img and returns them as a slice of rectangles that encapsulates them
func detectPersons(net *gocv.Net, img *gocv.Mat) []image.Rectangle {
	// convert img Mat to 672x384 blob that the face detector can analyze
	blob := gocv.BlobFromImage(*img, 1.0, image.Pt(672, 384), gocv.NewScalar(0, 0, 0, 0), false, false)
	defer blob.Close()

	// run a forward pass through the network
	net.SetInput(blob, "")
	results := net.Forward("")
	defer results.Close()

	// iterate through all detections and append results to persons buffer
	var persons []image.Rectangle
	for i := 0; i < results.Total(); i += 7 {
		confidence := results.GetFloatAt(0, i+2)
		if float64(confidence) > modelConfidence {
			left := int(results.GetFloatAt(0, i+3) * float32(img.Cols()))
			top := int(results.GetFloatAt(0, i+4) * float32(img.Rows()))
			right := int(results.GetFloatAt(0, i+5) * float32(img.Cols()))
			bottom := int(results.GetFloatAt(0, i+6) * float32(img.Rows()))
			persons = append(persons, image.Rect(left, top, right, bottom))
		}
	}

	return persons
}

// frameRunner reads image frames from framesChan and performs face and sentiment detections on them
// doneChan is used to receive a signal from the main goroutine to notify frameRunner to stop and return
func frameRunner(framesChan <-chan *frame, doneChan <-chan struct{}, resultsChan chan<- *Result,
	pubChan chan<- *Result, net *gocv.Net) error {

	// frame is image frame
	// we want to avoid continuous allocation that lead to GC pauses
	frame := new(frame)
	// result stores detection results
	result := new(Result)
	// perf is inference engine performance
	perf := new(Perf)
	for {
		select {
		case <-doneChan:
			fmt.Printf("Stopping frameRunner: received stop sginal\n")
			// close results channel
			close(resultsChan)
			// close publish channel
			if pubChan != nil {
				close(pubChan)
			}
			return nil
		case frame = <-framesChan:
			if frame == nil {
				continue
			}
			// let's make a copy of the original
			img := gocv.NewMat()
			frame.img.CopyTo(&img)

			persons := detectPersons(net, &img)

			alert := detectMotion(&img, persons, frame.area)

			perf = getPerformanceInfo(net)
			// detection result
			result = &Result{
				Alert: alert,
				Perf:  perf,
			}

			// send data down the channels
			resultsChan <- result
			if pubChan != nil {
				pubChan <- result
			}

			// close image matrices
			img.Close()
		}
	}

	return nil
}

func parseCliFlags() error {
	// parse cli flags
	flag.Parse()

	// path to pedestrian detection model can't be empty
	if model == "" {
		return fmt.Errorf("Invalid path to .bin file of pedestrian detection model: %s", model)
	}
	// path to pedestrian detection model config can't be empty
	if modelConfig == "" {
		return fmt.Errorf("Invalid path to .xml file of pedestrian model configuration: %s", modelConfig)
	}

	return nil
}

// NewInferModel reads DNN model and it configuration, sets its preferable target and backend and returns it.
// It returns error if either the model files failed to be read or setting the target fails
func NewInferModel(model, config string, backend, target int) (*gocv.Net, error) {
	// read in Face model and set the target
	m := gocv.ReadNet(model, config)

	if err := m.SetPreferableBackend(gocv.NetBackendType(backend)); err != nil {
		return nil, err
	}

	if err := m.SetPreferableTarget(gocv.NetTargetType(target)); err != nil {
		return nil, err
	}

	return &m, nil
}

// NewCapture creates new video capture from input or camera backend if input is empty and returns it.
// If input is not empty, NewCapture adjusts delay parameter so video playback matches FPS in the video file.
// It fails with error if it either can't open the input video file or the video device
func NewCapture(input string, deviceID int, delay *float64) (*gocv.VideoCapture, error) {
	if input != "" {
		// open video file
		vc, err := gocv.VideoCaptureFile(input)
		if err != nil {
			return nil, err
		}

		fps := vc.Get(gocv.VideoCaptureFPS)
		*delay = 1000 / fps

		return vc, nil
	}

	// open camera device
	vc, err := gocv.VideoCaptureDevice(deviceID)
	if err != nil {
		return nil, err
	}

	return vc, nil
}

// NewMQTTPublisher creates new MQTT client which collects analytics data and publishes them to remote MQTT server.
// It attempts to make a connection to the remote server and if successful it return the client handler
// It returns error if either the connection to the remote server failed or if the client config is invalid.
func NewMQTTPublisher() (*MQTTClient, error) {
	// create MQTT client and connect to MQTT server
	opts, err := MQTTClientOptions()
	if err != nil {
		return nil, err
	}

	// create MQTT client ad connect to remote server
	c, err := MQTTConnect(opts)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// SetRestrictedZone create a rectangle which encompasses restricted zone are in the image feed and returns it.
// If negative point coordinates are given, the area will default to the beginning of frame.
// If either default area size values are given (0,0) or negative ones we returned are defaults to the whole frame
func SetRestrictedZone(pointX, pointY, width, height int, img *gocv.Mat, area *image.Rectangle) {
	x, y := 0, 0
	w, h := width, height

	// if negative number given, we will default to the start of the frame
	if pointX > 0 && pointY > 0 {
		x = pointX
		y = pointY
	}

	// if either default values are given or negative we will default to the whole frame
	if width <= 0 {
		w = img.Cols()
	}

	if h <= 0 {
		h = img.Rows()
	}

	area.Min.X, area.Min.Y = x, y
	area.Max.X, area.Max.Y = x+w, y+h
}

// frame ise used to send video frames and program configuration to upstream goroutines
type frame struct {
	// img is image frame
	img *gocv.Mat
	// area is restricted zone area rectangle
	area *image.Rectangle
}

func main() {
	// parse cli flags
	if err := parseCliFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing command line parameters: %v\n", err)
		os.Exit(1)
	}

	// read in pedestrian detection model and set its inference backend and target
	net, err := NewInferModel(model, modelConfig, backend, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating pedestrian detection model: %v\n", err)
		os.Exit(1)
	}

	// create new video capture
	vc, err := NewCapture(input, deviceID, &delay)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating new video capture: %v\n", err)
		os.Exit(1)
	}
	defer vc.Close()

	// frames channel provides the source of images to process
	framesChan := make(chan *frame, 1)
	// errChan is a channel used to capture program errors
	errChan := make(chan error, 2)
	// doneChan is used to signal goroutines they need to stop
	doneChan := make(chan struct{})
	// resultsChan is used for detection distribution
	resultsChan := make(chan *Result, 1)
	// sigChan is used as a handler to stop all the goroutines
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
	// pubChan is used for publishing data analytics stats
	var pubChan chan *Result
	// waitgroup to synchronise all goroutines
	var wg sync.WaitGroup

	if publish {
		pub, err := NewMQTTPublisher()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create MQTT publisher: %v\n", err)
			os.Exit(1)
		}
		pubChan = make(chan *Result, 1)
		// start MQTT worker goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- messageRunner(doneChan, pubChan, pub, topic, rate)
		}()
		defer pub.Disconnect(100)
	}

	// start frameRunner goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- frameRunner(framesChan, doneChan, resultsChan, pubChan, net)
	}()

	// open display window
	window := gocv.NewWindow(name)
	window.SetWindowProperty(gocv.WindowPropertyAutosize, gocv.WindowAutosize)
	defer window.Close()

	// prepare input image matrix
	img := gocv.NewMat()
	defer img.Close()

	// initialize the result pointers
	result := new(Result)
	// restricted zone area
	var area image.Rectangle

monitor:
	for {
		if ok := vc.Read(&img); !ok {
			fmt.Printf("Cannot read image source %v\n", deviceID)
			break
		}
		if img.Empty() {
			continue
		}

		// get default restricted zone area
		SetRestrictedZone(pointX, pointY, width, height, &img, &area)
		// refine the are or quit the program
		switch key := window.WaitKey(int(delay)); key {
		// Select ROI
		case 99:
			// select ROI encompassing restricted zone
			roi := gocv.SelectROI(name, img)
			// update global configuration settings
			pointX, pointY = roi.Min.X, roi.Min.Y
			width, height = roi.Size().X, roi.Size().Y
			// update restricted zone area
			SetRestrictedZone(pointX, pointY, width, height, &img, &area)
			fmt.Printf("Restricted Zone: -x=%d -y=%d -height=%d -width=%d\n",
				area.Min.X, area.Min.Y, area.Size().X, area.Size().Y)
		// ESC button pressed
		case 27:
			fmt.Printf("Attempting to shut down: ESC pressed\n")
			break monitor
		}

		// draw restricted zone rectangle
		gocv.Rectangle(&img, area, color.RGBA{255, 0, 0, 0}, 1)
		// send data down the channel
		framesChan <- &frame{img: &img, area: &area}

		select {
		case sig := <-sigChan:
			fmt.Printf("Shutting down. Got signal: %s\n", sig)
			break monitor
		case err = <-errChan:
			fmt.Printf("Shutting down. Encountered error: %s\n", err)
			break monitor
		case result = <-resultsChan:
		default:
			// do nothing; just display latest results
		}
		// inference performance and print it
		gocv.PutText(&img, fmt.Sprintf("%s", result.Perf), image.Point{0, 15},
			gocv.FontHersheySimplex, 0.5, color.RGBA{0, 0, 0, 0}, 2)
		// inference results label
		gocv.PutText(&img, fmt.Sprintf("%s", result), image.Point{0, 40},
			gocv.FontHersheySimplex, 0.5, color.RGBA{0, 0, 0, 0}, 2)
		// display alert message when humane enters restricted zone
		if result.Alert {
			gocv.PutText(&img, alert, image.Point{0, 120},
				gocv.FontHersheySimplex, 0.5, color.RGBA{255, 0, 0, 0}, 2)
		}
		// show the image in the window, and wait 1 millisecond
		window.IMShow(img)
	}
	// signal all goroutines to finish
	close(framesChan)
	close(doneChan)
	// unblock resultsChan if necessary
	for range resultsChan {
		// collect any outstanding results
	}
	// wait for all goroutines to finish
	wg.Wait()
}
