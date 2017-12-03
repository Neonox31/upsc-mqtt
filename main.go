package main

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"bytes"
	"os/exec"
	"regexp"
	"os"
	"github.com/spf13/viper"
	"errors"
	"github.com/spf13/cast"
	"time"
	"github.com/op/go-logging"
)

var log	*logging.Logger

func initConfig() () {
	if (len(os.Args) < 2 || os.Args[1] == "") {
		log.Fatal("please specify a config file")
	}

	viper.SetConfigFile(os.Args[1])

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
}

func initLogging() () {
	log = logging.MustGetLogger("upsc-to-mqtt")
	format := logging.MustStringFormatter(
		`%{color}[%{level}] %{color:reset}%{message}`,
	)
	logging.SetFormatter(format)
	switch viper.GetString("log.level") {
	case "CRITICAL":
		logging.SetLevel(logging.CRITICAL, "")
		break
	case "ERROR":
		logging.SetLevel(logging.ERROR, "")
		break
	case "WARNING":
		logging.SetLevel(logging.WARNING, "")
		break
	case "NOTICE":
		logging.SetLevel(logging.NOTICE, "")
		break
	case "INFO":
		logging.SetLevel(logging.INFO, "")
		break
	case "DEBUG":
		logging.SetLevel(logging.DEBUG, "")
		break
	default:
		logging.SetLevel(logging.NOTICE, "")
	}
}

func publishOnMQTT(configValue map[string]interface{}, value string) (error) {
	topic := viper.GetString("mqtt.topic-level") + "/" + cast.ToString(configValue["id"])
	opts := MQTT.NewClientOptions()
	opts.AddBroker(viper.GetString("mqtt.uri"))
	opts.SetClientID(viper.GetString("mqtt.client-id"))

	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.WaitTimeout(time.Second * 5) && token.Error() != nil {
		return token.Error()
	}
	log.Debugf("publishing %s on %s", value, topic)
	token := c.Publish(topic, byte(cast.ToInt(configValue["mqtt-qos"])), cast.ToBool(configValue["mqtt-retained"]), value)
	token.WaitTimeout(time.Second * 5)
	c.Disconnect(250)
	return nil
}

func exposeWantedValues(values string) (error) {
	if (!viper.IsSet("values")) {
		return errors.New("please specify at least one value in config")
	}

	configValues := viper.Get("values").([]interface{})
	for _, table := range configValues {
		if configValue, err := table.(map[string]interface{}); err {
			if (cast.ToString(configValue["id"]) == "") {
				return errors.New("please specify an id for value")
			}
			if (cast.ToString(configValue["regex"]) == "") {
				return errors.New("please specify a regex for value")
			}
			valueRegex, _ := regexp.Compile(cast.ToString(configValue["regex"]))
			err := publishOnMQTT(configValue, valueRegex.FindStringSubmatch(values)[1])
			if (err != nil) {
				return err
			}
		}
	}
	return nil
}

func readUPSValues() (string, error) {
	command := exec.Command("/bin/upsc", "mge@localhost")
	var out bytes.Buffer
	command.Stdout = &out
	err := command.Run()
	if (err != nil) {
		return "", err
	}
	log.Debug(out.String())
	return out.String(), nil
}

func readValuesRoutine() {
	for {
		values, err := readUPSValues()
		if (err != nil) {
			log.Error(err)
		}
		err = exposeWantedValues(string(values))
		if (err != nil) {
			log.Error(err)
		}

		checkEvery := 60
		if (viper.IsSet("check-every")) {
			checkEvery = viper.GetInt("check-every")
		}

		time.Sleep(time.Duration(checkEvery) * time.Second)
	}
}

func init(){
	initConfig()
	initLogging()
}

func main() {
	readValuesRoutine()
}
