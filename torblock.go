package torblock

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const version = "0.1.0"
const timeFormat = "2006-01-02 15:04:05"

var checkURL = "https://check.torproject.org/exit-addresses"

type ExitAddress struct {
	IPAddress string
	Time      time.Time
}

type TorNode struct {
	ExitNode    string
	Published   time.Time
	LastStatus  time.Time
	ExitAddress ExitAddress
}

type TorNodeList struct {
	Nodes   []TorNode
	Updated time.Time
}

type Options struct {
	UpdateFrequency int64
	CheckURL        string
}

type TorBlock struct {
	opt            Options
	badHostHandler http.Handler
	NodeList       TorNodeList
}

func New(options Options) *TorBlock {
	return &TorBlock{
		opt:            options,
		badHostHandler: http.HandlerFunc(defaultBadHostHandler),
	}
}

func (t *TorBlock) Run() {

	if t.opt.CheckURL != "" {
		checkURL = t.opt.CheckURL
	}

	var updateFrequency int64
	updateFrequency = 3600 // default 1 hour

	if t.opt.UpdateFrequency > 0 {
		updateFrequency = t.opt.UpdateFrequency
	}

	frequency := time.Duration(updateFrequency) * time.Second
	ticker := time.NewTicker(frequency)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				list, err := fetchList()
				if err != nil {
					log.Println("[torblock] Failed to retrieve Tor node list")
				}
				t.NodeList = list
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (t *TorBlock) process(w http.ResponseWriter, r *http.Request) error {

	if len(t.NodeList.Nodes) == 0 {
		list, err := fetchList()
		if err != nil {
			log.Println("[torblock] Failed to retrieve Tor node list")
		}
		t.NodeList = list
	}

	if len(t.NodeList.Nodes) > 0 {
		isGoodHost := true
		for _, node := range t.NodeList.Nodes {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if strings.EqualFold(node.ExitAddress.IPAddress, host) {
				isGoodHost = false
				log.Printf("[torblock] Tor exit node detected: %s", host)
				break
			}
		}

		if !isGoodHost {
			t.badHostHandler.ServeHTTP(w, r)
			return fmt.Errorf("Bad host name: %s", r.Host)
		}
	}
	return nil
}

func fetchList() (TorNodeList, error) {
	var list TorNodeList
	response, err := http.Get(checkURL)
	if err != nil {
		return TorNodeList{}, err
	} else {
		// Now, some super awkward parsing for a super awkward document.
		log.Printf("[torblock] Retrieved list from %s", checkURL)
		defer response.Body.Close()
		contents := bufio.NewScanner(response.Body)
		var node TorNode
		var i = 0
		for contents.Scan() {
			if i%4 == 0 {
				if node.ExitNode != "" {
					list.Nodes = append(list.Nodes, node)
				}
				node = TorNode{}
			}
			parts := strings.Split(string(contents.Text()), " ")
			switch parts[0] {
			case "ExitNode":
				node.ExitNode = parts[1]
				break
			case "Published":
				pub, _ := time.Parse(timeFormat, parts[1]+" "+parts[2])
				node.Published = pub
				break
			case "LastStatus":
				status, _ := time.Parse(timeFormat, parts[1]+" "+parts[2])
				node.LastStatus = status
				break
			case "ExitAddress":
				exitseen, _ := time.Parse(timeFormat, parts[2]+" "+parts[3])
				node.ExitAddress.IPAddress = parts[1]
				node.ExitAddress.Time = exitseen
				break
			}
			i++
		}
		if err := contents.Err(); err != nil {
			return TorNodeList{}, err
		}
		list.Nodes = append(list.Nodes, node)
		return list, nil
	}
	return TorNodeList{}, errors.New("Could not retrieve list")
}

func (t *TorBlock) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := t.process(w, r)

		if err != nil {
			return
		}

		h.ServeHTTP(w, r)
	})
}

func (t *TorBlock) HandlerFuncWithNext(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	err := t.process(w, r)
	if err != nil {
		return
	}
	if next != nil {
		next(w, r)
	}
}
func defaultBadHostHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Bad Host", http.StatusInternalServerError)
}
