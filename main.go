package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-ping/ping"
	"github.com/joho/godotenv"
	"github.com/sfreiberg/simplessh"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/host/v3"
)

func main() {
	if err := godotenv.Load("./.env"); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	config := map[string]string{
		"HTTP_HOST":  "",
		"IDRAC_USER": "",
		"IDRAC_PASS": "",
		"IDRAC_HOST": "",
		"GPIO_PIN":   "",
	}
	for param, _ := range config {
		v := os.Getenv(param)
		if v == "" {
			log.Fatalf("Missing config param: %s", param)
		}
		config[param] = v
	}

	// Load all the periphio drivers:
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Lookup a pin by its number:
	p := gpioreg.ByName(config["GPIO_PIN"])
	if p == nil {
		log.Fatal("Failed to find", config["GPIO_PIN"])
	}

	log.Printf("%s: %s\n", p, p.(pin.PinFunc).Func())

	// Set it as input, with an internal pull down resistor:
	if err := p.In(gpio.PullDown, gpio.BothEdges); err != nil {
		log.Fatal(err)
	}

	// Wait for edges as detected by the hardware, and print the value read:
	lastSignal := p.Read()
	for {
		_ = p.WaitForEdge(-1)
		res := p.Read()
		if res == lastSignal {
			// the signal was the same, so ignore it
			continue
		}
		lastSignal = res

		log.Printf("triggered %s %s\n", res.String(), time.Now().
			Format(time.RFC3339))
		if res == gpio.Low {
			bootIfNotRunning(config)
		}
		// de-bounce the signal, a relay is slow compared to the sample rate
		time.Sleep(1 * time.Second)
	}
}

func bootIfNotRunning(config map[string]string) {
	resp, err := http.Get(config["HTTP_HOST"])
	if err != nil {
		log.Println("No http response; error: ", err, "(expected)")
	} else if resp.StatusCode == 200 {
		log.Println("already running")
		return
	}

	// if not, ping the lifcycle controller until it responds
	if err := pingOrTimeout(config["IDRAC_HOST"], 5*time.Minute); err != nil {
		log.Println("No ping response; error: ", err)
		return
	}

	// then log into the lifecycle controller and reboot
	client, err := simplessh.ConnectWithPassword(
		config["IDRAC_HOST"], config["IDRAC_USER"], config["IDRAC_PASS"])
	if err != nil {
		log.Fatalf("Failed to dial ssh to idrac: %v", err)
	}
	defer client.Close()

	// Now run the commands on the remote machine:
	b, err := client.Exec("racadm getsysinfo")
	if err != nil {
		log.Fatal(err)
	}

	if !powerStatusOff(b) {
		log.Print("Already powered on")
		return
	}
	log.Println("Powering on")
	_, err = client.Exec("racadm serveraction powerup")
	if err != nil {
		log.Fatal(err)
	}
}

func pingOrTimeout(host string, timeout time.Duration) error {
	firstPing := time.Now()
	for {
		if time.Since(firstPing) > timeout {
			log.Println("timed out waiting for ping response")
			return fmt.Errorf("timed out waiting for ping response")
		}
		pinger, err := ping.NewPinger(host)
		if err != nil {
			log.Println("No ping response; error: ", err, "waiting 1s")
			time.Sleep(1 * time.Second)
			continue
		}
		pinger.Count = 1
		if err := pinger.Run(); err != nil {
			log.Println("No ping response; error: ", err, "waiting 1s")
			time.Sleep(1 * time.Second)
			continue
		}
		log.Printf("ping statistics: %+v", pinger.Statistics())
		return nil
	}
}

func powerStatusOff(b []byte) bool {
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("Power Status")) {
			return !bytes.Contains(line, []byte("ON"))
		}
	}
	return false
}
