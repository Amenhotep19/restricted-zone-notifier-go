# Restricted Zone Notifier

| Details            |              |
|-----------------------|---------------|
| Target OS:            |  Ubuntu\* 16.04 LTS   |
| Programming Language: |  Go |
| Time to Complete:    |  45 min     |

![app image](./images/restricted-zone-notifier.png)

## Introduction

This restricted zone notifier application is one of a series of reference implementations for Computer Vision (CV) using the Intel® Distribution of OpenVINO™ toolkit written in the Go programming language. This example is designed for a machine mounted camera system that monitors if there are any humans present in a predefined selected assembly line area. It sends an alert if there is at least one person detected in the marked assembly area. The user can select the area coordinates either via command line parameters or once the application has been started they can select the region of interest (ROI) by pressing `ESC` key; this will pause the application, pop up a separate window on which the user can drag the mouse from the upper left ROI corner to whatever the size they require the area to cover. By default the whole frame is selected.

This example is intended to demonstrate how to use CV to improve assembly line safety for human operators and factory workers.

## Requirements

### Hardware

* 6th Generation Intel® Core™ processor with Intel® Iris® Pro graphics and Intel® HD Graphics

### Software

* [Ubuntu\* 16.04 LTS](http://releases.ubuntu.com/16.04/)
*Note*: You must be running kernel version 4.7+ to use this software. We recommend using a 4.14+ kernel to use this software. Run the following command to determine your kernel version:

```shell
uname -a
```

* OpenCL™ Runtime Package
* Intel® Distribution of OpenVINO™ toolkit
* Go programming language v1.11+

## Setup

### Install OpenVINO™ Toolkit

Refer to https://software.intel.com/en-us/articles/OpenVINO-Install-Linux for more information about how to install and setup the Intel® Distribution of OpenVINO™ toolkit.

You will need the OpenCL™ Runtime package if you plan to run inference on the GPU as shown by the
instructions below. It is not mandatory for CPU inference.

### Install Go

You must install the Go programming language version 1.11+ in order to compile this application. You can obtain the latest compiler from the Go website's download page at https://golang.org/dl/

For an excellent introduction to the Go programming language, check out the online tour at https://tour.golang.org

### Download the reference platform code using "go get"

You can download the reference platform code onto your computer by using the following Go command:

```shell
go get -d github.com/intel-iot-devkit/restricted-zone-notifier-go
```

Then, change the current directory to where you have installed the application code to continue the installation steps:

```shell
cd $GOPATH/src/github.com/intel-iot-devkit/restricted-zone-notifier-go
```

### Install Dep

This sample uses the `dep` dependency tool for Go. You can download and install it by running the following command:

```shell
make godep

```

### Install GoCV

Once you have installed Go, you must also install the GoCV (https://gocv.io/) package which contains the Go programming language wrappers for OpenVINO, and the associated dependencies. The easiest way to do this is by using the `dep` tool, which will satisfy the program's dependencies as defined in `Gopkg.lock` file. Run the following make file task to do so:

```shell
make dep
```

Now you should be ready to build and run the reference platform application code.

## How it Works

The application uses a video source, such as a camera, to grab frames, and then uses a Deep Neural Network (DNNs) to process the data. The network detects persons in the frame, and then if successful it checks if the detected persons are in the indicated off-limits assembly line region.

The data can then optionally be sent to a MQTT machine to machine messaging server, as part of an industrial data analytics system.

The DNN model used in this application is an Intel® optimized model that is part of the OpenVINO™ toolkit.

You can find it here:

- `/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002`

![Code organization](./images/arch3.png)

The program creates three go routines for concurrency:

- Main goroutine that performs the video i/o
- Worker goroutine that processes video frames using the deep neural networks
- Worker goroutine that publishes MQTT messages to remote server

## Setting the Build Environment

You must configure the environment to use the OpenVINO™ toolkit one time per session by running the following command:

```shell
source /opt/intel/computer_vision_sdk/bin/setupvars.sh
```

## Building the Code

Start by changing the current directory to wherever you have git cloned the application code. For example:

```shell
cd $GOPATH/src/github.com/intel-iot-devkit/restricted-zone-notifier-go
```

Before you can build the program you need to fetch its dependencies. You can do that by running the commands below. The first one fetches `Go` depedency manager of our choice and the latter uses it to satisfy the program's depdencies as defined in `Gopkg.lock` file:

```shell
make godep
make dep
```

Once you have fetched the dependencies you must export a few environment variables required to build the library from the fetched dependencies. Run the following command from the project directory:

```shell
source vendor/gocv.io/x/gocv/openvino/env.sh
```

Now you are ready to build the program binary. The project ships a simple `Makefile` which makes building the program easy by invoking the `build` task from the project root as follows:

```shell
make build
```

This commands creates a new directory called `build` in your current working directory and places the newly built binary called `notifier` into it.
Once the commands are finished, you should have built the `notifier` application executable.

## Running the Code

To see a list of the various options:

```shell
cd build
./notifier -help
```

To run the application with the needed model using the webcam:

```shell
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.xml
```

You can select an area to be used as the "off-limits" area by pressing the `c` key once the program is running. A new window will open showing a still image from the video capture device. Drag the mouse from left top corner to cover an area on the plane and once done (a blue rectangle is drawn) present `ENTER` or `SPACE` to proceed with notifiering.

Once you have selected the "off-limits" area the coordinates will be displayed in the terminal window like this:

```shell
Restricted Zone: -x=429 -y=101 -height=619 -width=690
```

You can run the application using those coordinates by using the `-x`, `-y`, `-height`, and `-width` flags to pre-select that area.

For example:

```shell
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.xml -x=429 -y=101 -height=619 -width=690
```

If you do not select or specify an area, the default is to use the entire window as the off limits area.

### Hardware Acceleration

This application can take advantage of the hardware acceleration in the OpenVINO toolkit by using the `-backend, -b` and `-target, -t` parameters.

For example, to use the OpenVINO™ toolkit backend with the GPU in 32-bit mode:

```shell
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.xml -backedn=2 -target=1
```

To run the code using 16-bit floats, you have to both set the `-target` flag to use the GPU in 16-bit mode, as well as use the FP16 version of the Intel® models:

```shell
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP16/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP16/pedestrian-detection-adas-0002.xml -backend=2 -target=2
```

To run the code using the VPU, you have to set the `-target` flag to `3` and also use the 16-bit FP16 version of the Intel® models:

```shell
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP16/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP16/pedestrian-detection-adas-0002.xml -backend=2 -target=3
```

## Sample Videos

There are several videos available to use as sample videos to show the capabilities of this application. You can download them by running these commands from the `restricted-zone-notifier-go` directory:

```shell
mkdir resources
cd resources
wget https://github.com/intel-iot-devkit/sample-videos/raw/master/worker-zone-detection.mp4
cd ..
```

To then execute the code using one of these sample videos, run the following commands from the `restricted-zone-notifier-go` directory:

```shell
cd build
./notifier -model=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.bin -model-config=/opt/intel/computer_vision_sdk/deployment_tools/intel_models/pedestrian-detection-adas-0002/FP32/pedestrian-detection-adas-0002.xml -input=../resources/worker-zone-detection.mp4 -model-confidence=0.7
```

### Machine to Machine Messaging with MQTT

If you wish to use a MQTT server to publish data, you should set the following environment variables before running the program and use `-publish` flag when launching the program:

```shell
export MQTT_SERVER=localhost:1883
export MQTT_CLIENT_ID=cvservice
```

Change the `MQTT_SERVER` to a value that matches the MQTT server you are connecting to.

You should change the `MQTT_CLIENT_ID` to a unique value for each monitoring station, so you can track the data for individual locations. For example:

```shell
export MQTT_CLIENT_ID=zone1337
```

If you want to monitor the MQTT messages sent to your local server, and you have the `mosquitto` client utilities installed, you can run the following command:

```shell
mosquitto_sub -t 'machine/zone'
```

## Docker

You can also build a Docker image and then run the program in a Docker container. First you need to build the image. You can use the `Dockerfile` present in the cloned repository and build the Docker image.

First you must obtain your own unique download URL for the Intel distribution of OpenVINO toolkit. Follow the registration process if you have not yet done so. In the registration email you have received a link to the Intel Registration Center website download page, shown here:

![OpenVINO download page](./images/openvino-download.png)

First, navigate to the download page using the link you have received. On the download page, use the "Choose Product to Download" selection box and select "Intel Distribution of OpenVINO toolkit for Linux". Next, using the "Choose a Version" selection box, select "2018 R5". The "Choose a Download Option" section should appear. Right click on the button "Full Package" and choose "Copy Link Address". Your clipboard should now contain your unique OpenVINO download URL. Save this URL somewhere safe.

Now you can build your unique Docker image by running the following command, substituting the actual URL you obtained in the previous step:

```shell
docker build -t restricted-zone-notifier-go --build-arg OPENVINO_DOWNLOAD_URL=[your unique OpenVINO download URL here] .
```

This will produce a docker image called `restricted-zone-notifier-go` which contains the built binary. Since the built docker image has an [ENTRYPOINT](https://docs.docker.com/engine/reference/builder/#entrypoint) defined you can run the image as an executable using the following command:

```shell
docker run -it --rm restricted-zone-notifier-go -h
```

To run the docker image on an Ubuntu host machine using an attached camera, run the following commands:

```shell
xhost +local:docker
docker run --device=/dev/video0:/dev/video0 -v /tmp/.X11-unix:/tmp/.X11-unix -e DISPLAY=$DISPLAY -it --rm restricted-zone-notifier-go 
xhost -local:docker
```

To run the docker image on an Ubuntu host machine using a file input, run the following commands:

```shell
xhost +local:docker
docker run -v ${PWD}/resources:/resources -v /tmp/.X11-unix:/tmp/.X11-unix -e DISPLAY=$DISPLAY -it --rm restricted-zone-notifier-go -input=/resources/face-demographics-walking-and-pause.mp4
xhost -local:docker
```

## Microsoft Azure

If you'd like to know how you can take advantage of more advanced build system provided by [Microsoft Azure Cloud](https://azure.microsoft.com/), please check out the Azure guide [here](./azure.md). Following the steps in the guide you can build a Docker container and push it into Azure Container Registry to make it available online.
