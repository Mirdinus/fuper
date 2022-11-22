package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Message struct {
	Ip              string `json:"ip"`
	Transfered      int    `json:"transfered"`
	CanBeTransfered int    `json:"canBeTransfered"`
	Percent         int    `json:"percent"`
	MinSpeed        int    `json:"minSpeed"`
	MaxSpeed        int    `json:"maxSpeed"`
	// "transfered in MB and speeds in kb/s"
}

var promPerc = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "fuper",
	Subsystem: "mladeth",
	Name:      "percentage",
})

var promCanTran = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "fuper",
	Subsystem: "mladeth",
	Name:      "can_transfer",
})

var promWasTran = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "fuper",
	Subsystem: "mladeth",
	Name:      "transfered",
})

var promMinSpeed = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "fuper",
	Subsystem: "mladeth",
	Name:      "min_speed",
})

var promMaxSpeed = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "fuper",
	Subsystem: "mladeth",
	Name:      "max_speed",
})

func loadData() ([]byte, error) {

	url := "http://212.27.205.129/"
	re := strings.NewReplacer("<!--", "", "-->", "", "\n", "")

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	buf := new(strings.Builder)
	io.Copy(buf, res.Body)

	cleanBody := re.Replace(buf.String())

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(cleanBody))

	var msg Message

	doc.Find("table").Find("tr").Each(
		func(i int, s *goquery.Selection) {
			tds := s.Find("td")

			key := tds.First().Text()
			value := tds.Last().Text()

			if key == value {
				return
			}

			value = strings.Fields(value)[0]

			switch key {
			case "IP adresa":
				msg.Ip = value
			case "Přeneseno dat":
				msg.Transfered, _ = strconv.Atoi(value)
			case "Kvóta":
				msg.CanBeTransfered, _ = strconv.Atoi(value)
			case "Využití kvóty":
				msg.Percent, _ = strconv.Atoi(value)
			case "Minimální zaručená rychlost":
				msg.MinSpeed, _ = strconv.Atoi(value)
			case "Maximální omezená rychlost":
				msg.MaxSpeed, _ = strconv.Atoi(value)
			}
		})

	promPerc.Set(float64(msg.Percent))
	promCanTran.Set(float64(msg.CanBeTransfered))
	promWasTran.Set(float64(msg.Transfered))
	promMinSpeed.Set(float64(msg.MinSpeed))
	promMaxSpeed.Set(float64(msg.MaxSpeed))

	jsonMapAsStringFormat, _ := json.Marshal(msg)

	log.Println("Data loaded")
	return jsonMapAsStringFormat, nil
}

var fuperData []byte

func load() {
	toReturn, err := loadData()
	if err != nil {
		log.Println(err)
	}
	fuperData = toReturn
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Data requested via HTTP")
		w.Header().Set("Content-Type", "application/json")
		w.Write(fuperData)
	})

	ticker := time.NewTicker(10 * time.Minute)

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	prometheus.MustRegister(promPerc)
	prometheus.MustRegister(promCanTran)
	prometheus.MustRegister(promWasTran)
	prometheus.MustRegister(promMinSpeed)
	prometheus.MustRegister(promMaxSpeed)

	go func() {
		for {
			select {
			case <-done:
				os.Exit(1)
			case <-ticker.C:
				load()
			}
		}
	}()

	load()

	log.Println("Server started")
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":5050", nil)

}
