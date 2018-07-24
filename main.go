// This program currently just drops OpenChirp device transducer values
// into Redis.
// The future goal of this program will be to manage all MQTT bridging and magic
// for the OpenChirp core.
// Craig Hesling <craig@hesling.com>
package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/openchirp/framework/pubsub"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/wercker/journalhook"
)

const (
	mqttQoS = pubsub.QoSExactlyOnce
)

var (
	lastValueExpiration = time.Hour * time.Duration(24*32*4) // roughly 4 months
	version             = "1.0"
)

func run(ctx *cli.Context) error {
	systemdIntegration := ctx.Bool("systemd")

	/* Set logging level */
	log := logrus.New()
	log.SetLevel(logrus.Level(uint32(ctx.Int("log-level"))))
	if systemdIntegration {
		log.AddHook(&journalhook.JournalHook{})
		log.Out = ioutil.Discard
	}

	/* Argument Receiving */
	mqttBroker := ctx.String("mqtt-server")
	mqttUser := ctx.String("mqtt-user")
	mqttPass := ctx.String("mqtt-pass")
	redisAddr := ctx.String("redis-server")
	redisPassword := ctx.String("redis-password")
	redisDB := ctx.Int("redis-db")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	defer redisClient.Close()
	pong, err := redisClient.Ping().Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis: ", err)
	}
	log.Debug("Redis pong: ", pong)

	mqttClient, err := pubsub.NewMQTTClient(mqttBroker, mqttUser, mqttPass, mqttQoS, false)
	if err != nil {
		log.Fatal("Failed to connect to MQTT Broker: ", err)
	}
	defer mqttClient.Disconnect()

	mqttClient.Subscribe("openchirp/device/+/+", func(topic string, payload []byte) {
		log.Debugf("MQTT Message: %s = %s", topic, string(payload))

		// Apply standard OpenChirp topic rules
		key := strings.Replace(topic, "/", ":", 3)
		key = strings.Replace(key, " ", "_", 3)
		key = strings.ToLower(key)
		value := string(payload)

		res, err := redisClient.Set(key, value, lastValueExpiration).Result()
		if err != nil {
			log.Errorf("Failed to set %s with %s: response=%v | err=%v", key, value, res, err)
		}
	})

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)
	<-sig

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "openchirp_pubsub"
	app.Usage = ""
	app.Copyright = "See https://github.com/openchirp/openchirp_pubsub for copyright information"
	app.Author = "Craig Hesling"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "MQTT server's URI (e.g. scheme://host:port where scheme is tcp or tls)",
			Value:  "tls://localhost:8883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-user",
			Usage:  "Username to login to the MQTT server with",
			EnvVar: "MQTT_USER",
		},
		cli.StringFlag{
			Name:   "mqtt-pass",
			Usage:  "Password to login to the MQTT server with",
			EnvVar: "MQTT_PASS",
		},
		cli.StringFlag{
			Name:   "redis-server",
			Value:  "localhost:6379",
			Usage:  "The URI to the Redis server",
			EnvVar: "REDIS_SERVER",
		},
		cli.StringFlag{
			Name:   "redis-pass",
			Value:  "",
			Usage:  "Password to login to the Redis server with",
			EnvVar: "REDIS_PASS",
		},
		cli.IntFlag{
			Name:   "redis-db",
			Value:  1,
			Usage:  "Selects which Redis DB to use",
			EnvVar: "REDIS_DB",
		},
		cli.IntFlag{
			Name:   "log-level",
			Value:  4,
			Usage:  "debug=5, info=4, warning=3, error=2, fatal=1, panic=0",
			EnvVar: "LOG_LEVEL",
		},
		cli.BoolFlag{
			Name:   "systemd",
			Usage:  "Indicates that this service can use systemd specific interfaces.",
			EnvVar: "SYSTEMD",
		},
	}
	app.Run(os.Args)
}
