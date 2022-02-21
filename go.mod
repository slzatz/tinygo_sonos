module github.com/slzatz/tinygo_sonos

go 1.17

replace tinygo.org/x/drivers v0.19.0 => /home/slzatz/drivers

require (
	github.com/mailru/easyjson v0.7.7
	tinygo.org/x/drivers v0.19.0
	tinygo.org/x/tinyfont v0.2.1
)

require (
	github.com/eclipse/paho.mqtt.golang v1.2.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
)
