package main

// combines mqtt and writing to the epaper display
// each of those things works separately but
// json seems to send it over edge

import (
	"image/color"
	"machine"
	"time"

	"github.com/mailru/easyjson"
	"tinygo.org/x/drivers/net/mqtt"
	"tinygo.org/x/drivers/waveshare-epd/epd4in2"
	"tinygo.org/x/drivers/wifinina"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

/*
const ssid = ""
const pass = ""
*/

var (
	// these are the default pins for the Arduino Nano33 IoT.
	spi     = machine.NINA_SPI
	adaptor *wifinina.Device
	cl      mqtt.Client

	display epd4in2.Device
	black   = color.RGBA{1, 1, 1, 255}
	font    = &freemono.Bold9pt7b
)

//var topic = "sonos/current_track"
const topic = "sonos/current_track"

//easyjson:json
type JSONData struct {
	Artist string
	Title  string
}

//const server = "tcp://...:1883"

//var topic = "sonos/current_track"
//const topic = "sonos/current_track"

func subHandler(client mqtt.Client, msg mqtt.Message) {
	d := &JSONData{}
	err := easyjson.Unmarshal(msg.Payload(), d)
	if err != nil {
		println("easyjson.Unmarshal: ", err)
	}
	println("artist: ", d.Artist)
	println("track: ", d.Title)
	display.ClearDisplay()
	display.ClearBuffer()
	time.Sleep(3000 * time.Millisecond)                                                     // needs min ~3 sec
	tinyfont.WriteLineRotated(&display, font, 2, 20, d.Artist, black, tinyfont.NO_ROTATION) //x,y
	tinyfont.WriteLineRotated(&display, font, 2, 60, d.Title, black, tinyfont.NO_ROTATION)
	time.Sleep(1500 * time.Millisecond) // needs min ~1.5 sec
	display.Display()
}

func main() {
	err := machine.SPI0.Configure(machine.SPIConfig{Frequency: 2000000}) //115200 worked
	if err != nil {
		println(err)
	}
	// these could have been any digital except SPI D11-D13
	busyPin := machine.D7
	rstPin := machine.D8
	dcPin := machine.D9  //Data/Command: high for data, low for command
	csPin := machine.D10 //CS - low active
	//dinPin := machine.D11 //SDO //MOSI //D12 MISO/SDI
	//d12 doesn't appear that waveshare uses SDI/MISO
	//clkPin := machine.D13 //sck

	var config epd4in2.Config
	config.Width = 200        // 400 // 150
	config.Height = 150       // 300 // 100
	config.LogicalWidth = 200 // 400
	config.Rotation = 0

	display = epd4in2.New(machine.SPI0, csPin, dcPin, rstPin, busyPin)
	display.Configure(config)
	time.Sleep(3000 * time.Millisecond)
	display.ClearDisplay()

	// Configure SPI for 8Mhz, Mode 0, MSB First
	spi.Configure(machine.SPIConfig{
		Frequency: 8 * 1e6,
		SDO:       machine.NINA_SDO,
		SDI:       machine.NINA_SDI,
		SCK:       machine.NINA_SCK,
	})

	// Init wifit
	adaptor = wifinina.New(spi,
		machine.NINA_CS,
		machine.NINA_ACK,
		machine.NINA_GPIO0,
		machine.NINA_RESETN)
	adaptor.Configure()

	connectToAP()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(server).SetClientID("tinygo-client-1")

	println("Connecting to MQTT broker at", server)
	cl = mqtt.NewClient(opts)
	token := cl.Connect()

	if token.Wait() && token.Error() != nil {
		failMessage("mqtt connect", token.Error().Error())
	}

	// subscribe
	println("Subscribing ...")
	token = cl.Subscribe(topic, 0, subHandler)
	token.Wait()
	if token.Error() != nil {
		failMessage("mqtt subscribe", token.Error().Error())
	}

	for {
		token := cl.Pingreq()
		if token.Error() != nil {
			failMessage("ping", token.Error().Error())
		}
		time.Sleep(30 * time.Second)
	}
}

func failMessage(action, msg string) {
	println(action, ": ", msg)
	time.Sleep(5 * time.Second)
}

func connectToAP() {
	time.Sleep(2 * time.Second)
	println("Connecting to " + ssid)
	err := adaptor.ConnectToAccessPoint(ssid, pass, 10*time.Second)
	if err != nil { // error connecting to AP
		for {
			println(err)
			time.Sleep(1 * time.Second)
		}
	}

	println("Connected.")

	time.Sleep(2 * time.Second)
	ip, _, _, err := adaptor.GetIP()
	for ; err != nil; ip, _, _, err = adaptor.GetIP() {
		println(err.Error())
		time.Sleep(1 * time.Second)
	}
	println(ip.String())
}
