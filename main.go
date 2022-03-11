package main

/*
combines mqtt and writing to the epaper display
each of those things works separately but
need to limit epaper canvas size to get
it to work with Arduino Nano 33 IoT and
Arduino MKR Wifi 1010 which have
256kb of flash and 32kb of SRAM
*/

import (
	"fmt"
	"image/color"
	"machine"

	"math/rand"
	"strings"
	"time"

	"github.com/mailru/easyjson"
	"tinygo.org/x/drivers/net/mqtt"
	"tinygo.org/x/drivers/waveshare-epd/epd4in2"
	"tinygo.org/x/drivers/wifinina"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

var (
	bat     = machine.ADC{machine.A6}
	spi     = machine.NINA_SPI
	adaptor *wifinina.Device
	cl      mqtt.Client

	display epd4in2.Device
	black   = color.RGBA{1, 1, 1, 255}
	font    = &freemono.Bold9pt7b
	Board   string
)

const topic = "sonos/current_track"

//easyjson:json
type JSONData struct {
	Artist string
	Title  string
}

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
	time.Sleep(3000 * time.Millisecond) // needs min ~3 sec
	var line int16
	line = writeString(d.Artist, 19, 20)
	_ = writeString(d.Title, 19, line+25)
	v := bat.Get()
	vv := float32(v) * 3.3 * 1.1 * 2 / 65536 // 1.1 is just a kluge to get closer to expected battery at full charge
	voltage := fmt.Sprintf("VBat: %.2f (%x)", vv, v)
	tinyfont.WriteLineRotated(&display, font, 2, 250, voltage, black, tinyfont.NO_ROTATION)
	println(voltage)
	time.Sleep(1500 * time.Millisecond) // needs min ~1.5 sec
	display.Display()
}

func writeString(s string, ln int, line int16) int16 {
	if len(s) < ln {
		tinyfont.WriteLineRotated(&display, font, 2, line, s, black, tinyfont.NO_ROTATION)
		return line
	} else {
		ss := strings.Split(s, " ")
		n := len(ss) - len(ss)/2
		firstLine := strings.Join(ss[:n], " ")
		secondLine := strings.Join(ss[n:], " ")
		tinyfont.WriteLineRotated(&display, font, 2, line, firstLine, black, tinyfont.NO_ROTATION)
		line += 15
		tinyfont.WriteLineRotated(&display, font, 2, line, secondLine, black, tinyfont.NO_ROTATION)
		return line
	}
}

func main() {
	// configure battery reading
	machine.InitADC()
	bat.Configure(machine.ADCConfig{Samples: 4, Reference: 3300}) // 3.3 volts is default but setting explicitly
	// below for epd
	err := machine.SPI0.Configure(machine.SPIConfig{Frequency: 2000000}) //115200 worked
	if err != nil {
		println(err)
	}
	println("Board:", Board)
	var busy, rst, dc, cs machine.Pin
	if Board[0] == 0x6e { // using -ldflags="-X 'main.Board=n'"
		/* below are pins used for the nano 33 IoT*/
		// these could have been any digital pins except SPI D11-D13
		busy = machine.D7
		rst = machine.D8
		dc = machine.D9  //Data/Command: high for data, low for command
		cs = machine.D10 //CS - low active
		//	din := machine.D11 => SDO/MOSI //D12 MISO/SDI
		//	d12 SDI/MISO; not used by waveshare -- assume shouldn't be used for something else
		//	clk := machine.D13 => SCK
	} else if Board[0] == 0x6d { // -ldflags="-X 'main.Board=m'"
		busy = machine.D6
		rst = machine.D5
		dc = machine.D4 //22 //Data/Command: high for data, low for command
		cs = machine.D9 //21 //CS - low active

	} else {
		/* below are the pins used for the mkr wifi 1010 */
		// these could have been any digital pins except SPI D8-D10
		busy = machine.D2
		rst = machine.D3
		dc = machine.D4 //Data/Command: high for data, low for command
		cs = machine.D5 //CS - low active
		//	din := machine.D8 => SDO/MOSI //D12 MISO/SDI
		//	d10 SDI/MISO; not used by waveshare -- assume shouldn't be used for something else
		//	clk := machine.D9 => SCK
	}

	var config epd4in2.Config
	config.Width = 400        // 200
	config.Height = 300       // 150
	config.LogicalWidth = 400 // 200
	config.Rotation = 0

	display = epd4in2.New(machine.SPI0, cs, dc, rst, busy)
	display.Configure(config)
	time.Sleep(3000 * time.Millisecond)
	display.ClearDisplay()
	//println(busy)
	//println(rst)

	// Configure SPI for 8Mhz, Mode 0, MSB First
	spi.Configure(machine.SPIConfig{
		Frequency: 8 * 1e6,
		SDO:       machine.NINA_SDO,
		SDI:       machine.NINA_SDI,
		SCK:       machine.NINA_SCK,
	})

	time.Sleep(5 * time.Second) ///////
	// Init wifit
	adaptor = wifinina.New(spi,
		machine.NINA_CS,
		machine.NINA_ACK,
		machine.NINA_GPIO0,
		machine.NINA_RESETN,
	)
	//adaptor.Configure()
	adaptor.Configure2(false)   //true = reset active high
	time.Sleep(5 * time.Second) // necessary
	s, err := adaptor.GetFwVersion()
	if err != nil {
		println("firmware:", err)
	}
	println("firmware:", s)

	//time.Sleep(10 * time.Second) ///////

	for {
		err := connectToAP()
		if err == nil {
			break
		}
	}

	opts := mqtt.NewClientOptions()
	clientID := "tinygo-client-" + randomString(len(Board))
	opts.AddBroker(server).SetClientID(clientID)
	println(clientID)
	//opts.AddBroker(server).SetClientID("tinygo-client-2")

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

func connectToAP() error {
	time.Sleep(2 * time.Second)
	println("Connecting to " + ssid)
	err := adaptor.ConnectToAccessPoint(ssid, pass, 10*time.Second)
	if err != nil {
		println(err)
		//time.Sleep(2 * time.Second)
		return err
	}

	println("Connected.")

	time.Sleep(2 * time.Second)
	ip, _, _, err := adaptor.GetIP()
	for ; err != nil; ip, _, _, err = adaptor.GetIP() {
		println(err.Error())
		time.Sleep(1 * time.Second)
	}
	println(ip.String())
	return nil
}

// Returns an int >= min, < max
func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate a random string of A-Z chars with len = l
func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(65, 90))
	}
	return string(bytes)
}
