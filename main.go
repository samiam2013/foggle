package main

import (
	"bytes"
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

	httpHost := os.Getenv("HTTP_HOST")
	idracUser := os.Getenv("IDRAC_USER")
	idracPass := os.Getenv("IDRAC_PASS")
	idracHost := os.Getenv("IDRAC_HOST")
	if httpHost == "" || idracUser == "" || idracPass == "" || idracHost == "" {
		log.Fatal("missing env vars, (IDRAC|HTTP)_HOST, IDRAC_(USER|PASS) must be set")
	}

	// Load all the drivers:
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Lookup a pin by its number:
	pName := "GPIO27"
	p := gpioreg.ByName(pName)
	if p == nil {
		log.Fatal("Failed to find", pName)
	}

	log.Printf("%s: %s\n", p, p.(pin.PinFunc).Func())

	// Set it as input, with an internal pull down resistor:
	if err := p.In(gpio.PullDown, gpio.BothEdges); err != nil {
		log.Fatal(err)
	}

	// Wait for edges as detected by the hardware, and print the value read:
	lastSignal := gpio.Low
	for {
		p.WaitForEdge(-1)
		res := p.Read()
		if res != lastSignal {
			log.Printf("triggered %s %s\n", res.String(),
				time.Now().Format(time.RFC3339))
			if res == gpio.Low {
				bootIfNotRunning(httpHost, idracUser, idracPass, idracHost)
			}
			lastSignal = res
			// de-bounce
			time.Sleep(1 * time.Second)
		}
	}
}

func bootIfNotRunning(httpHost, user, pass, idracHost string) {
	resp, err := http.Get(httpHost)
	if err != nil {
		log.Println("error: ", err)
		return
	}
	if resp.StatusCode == 200 {
		log.Println("already running")
		return
	}

	// if not, ping the lifcycle controller until it responds
	firstPing := time.Now()
	for {
		if time.Since(firstPing) > 5*time.Minute {
			log.Println("timed out waiting for ping response")
			return
		}
		pinger, err := ping.NewPinger(idracHost)
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
		break
	}

	// then log into the lifecycle controller and reboot
	log.Println("rebooting")
	client, err := simplessh.ConnectWithPassword(idracHost, user, pass)
	if err != nil {
		log.Fatalf("Failed to dial ssh to idrac: %v", err)
	}

	defer client.Close()

	// Now run the commands on the remote machine:
	b, err := client.Exec("racadm getsysinfo")
	if err != nil {
		log.Fatal(err)
	}

	if powerStatusOff(b) {
		log.Println("powering on")
		_, err = client.Exec("racadm serveraction powerup")
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Print("already powered on")
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
