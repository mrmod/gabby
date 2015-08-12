package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

/*
Gabby is a simple peer cluster. Data-leader is
a one time event established by start-up time.

The data must not be lost once started but the
initial data can be compiled multiple times prior
to selecting the data-leader in the event there
are start-up time collisions
*/

type Node struct {
	startTime int64
	name      string
	content   []string
	leader    *Node
	peers     []*Node
}

func NewNode() *Node {
	port := os.Getenv("PORT")
	host := os.Getenv("HOSTNAME")
	peers := os.Getenv("PEERS")
	name := host + ":" + port
	t := time.Now().UnixNano()

	node := &Node{
		t + rand.Int63n(t),
		name,
		[]string{},
		nil,
		[]*Node{},
	}

	if len(peers) > 0 {
		peerList := strings.Split(peers, ",")
		node.initPeers(peerList...)
	}
	return node
}

func NewPeer(peerHostPort string) *Node {
	return &Node{int64(-1), peerHostPort, []string{}, nil, []*Node{}}
}

func (n *Node) Start() {
	fmt.Printf("Starting node %s with %d peers\n", n.name, len(n.peers))
	go n.startWeb()

	if len(n.peers) == 0 {
		fmt.Println("No peers,", n.name, " is the leader")
		// Generate content
	} else {
		fmt.Println("Waiting five seconds for other peers")
		time.Sleep(time.Second * 5)
		fmt.Println("Electing leader")
		n.consensus()
		fmt.Println("Fetching leader content")
		n.leaderContent()
	}
}

func (n *Node) startWeb() {
	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/startup", func(r render.Render) {
		r.JSON(200, map[string]int64{n.name: n.startTime})
	})
	m.Get("/content", func(r render.Render) {
		r.JSON(200, n.content)
	})
	m.Get("/election", func(r render.Render) {
		n.startTime = n.startTime + rand.Int63n(n.startTime)
		r.JSON(200, map[string]int64{n.name: n.startTime})
	})
	m.Get("/leader", func(r render.Render) {
		if n.leader == nil {
			r.JSON(200, "")
		} else {
			r.JSON(200, n.leader.name)
		}
	})
	m.Get("/ping", func(r render.Render) {
		r.JSON(200, n.name)
	})

	m.Run()
}

func (n *Node) leaderContent() {
	contentResponse := n.get(n.leader, "/content")
	select {
	case b := <-contentResponse:
		fmt.Println("Setting content: ", string(b))
		if err := json.Unmarshal(b, &n.content); err != nil {
			fmt.Println("Failed to set content: ", err)
		}
	}
}

func (n *Node) electLeader() {
	var wg sync.WaitGroup
	wg.Add(len(n.peers))
	peerResponses := make([]chan []byte, len(n.peers))

	for _, peer := range n.peers {
		peerResponses = append(peerResponses, n.get(peer, "/startup"))
	}

	for _, response := range peerResponses {
		go func(r <-chan []byte) {
			name, startTime := startupResponse(<-r)
			n.setPeerStartTime(name, startTime)
			wg.Done()
		}(response)
	}

	wg.Wait()
	n.leader = n.firstStarter()
}

func (n *Node) newElection() {
	var wg sync.WaitGroup
	wg.Add(len(n.peers))
	peerResponses := make([]chan []byte, len(n.peers))

	for _, peer := range n.peers {
		peerResponses = append(peerResponses, n.get(peer, "/election"))
	}

	for _, response := range peerResponses {
		go func(r <-chan []byte) {
			name, startTime := startupResponse(<-r)
			n.setPeerStartTime(name, startTime)
			wg.Done()
		}(response)
	}

	wg.Wait()
}

func (n *Node) consensus() {
	n.electLeader()
	var wg sync.WaitGroup
	wg.Add(len(n.peers))
	peerResponses := make([]chan []byte, len(n.peers))

	for _, peer := range n.peers {
		peerResponses = append(peerResponses, n.get(peer, "/leader"))
	}
	haveConsensus := true
	for _, response := range peerResponses {
		go func(r <-chan []byte) {
			name := leader(<-r)
			if name != n.leader.name {
				haveConsensus = false
			}
			wg.Done()
		}(response)
	}
	wg.Wait()

	if !haveConsensus {
		fmt.Println("Starting a new election")
		n.newElection()
		n.consensus()
	}
	fmt.Printf("***[%s] Elected %s as the leader\n", n.name, n.leader.name)
}

// maintainConsensus Ping the master to ensure it's still the master.
// If the master cannot be reached, ask the other members to check if
// they can ping the leader. If no members can reach the leader,
// hold a new election. If we can reach no members, we're the problem
// and sleep wait until all members we know about have pinged us us
// to rejoin
func (n *Node) maintainConsensus() {
	if n.leader == nil {
		// TODO: Should we find a new leader?
		return
	}

	leaderResponse := n.get(n.leader, "/ping")
	select {
	case r := <-leaderResponse:
		if len(r) == 0 {
			n.newElection()
		}
	}
}

// maintainPeers Pings all peers and removes any unresponsive ones
func (n *Node) maintainPeers() {
	peerResponses := make([]chan []byte, len(n.peers))

	// TODO: The fan-out/fan-in is so common, let's generalize
	for _, peer := range n.peers {
		peerResponses = append(peerResponses, n.get(peer, "/ping"))
	}

	livePeers := []string{}
	for _, response := range peerResponses {
		go func(r <-chan []byte) {
			if ping := <-r; len(ping) > 0 {
				livePeers = append(livePeers, string(ping))
			}
		}(response)
	}

	livePeerList := []*Node{}
	for _, peerName := range livePeers {
		if found := n.findPeer(peerName); found != nil {
			livePeerList = append(livePeerList, found)
		}
	}
	// TODO: Could diff the peers here for a helpful log message
	n.peers = livePeerList
}

func (n *Node) firstStarter() *Node {
	lowestPeer := n
	for _, peer := range n.peers {
		if peer.startTime < lowestPeer.startTime {
			lowestPeer = peer
		}
	}
	return lowestPeer
}

func (n *Node) setPeerStartTime(name string, i int64) {
	if peer := n.findPeer(name); peer != nil {
		peer.startTime = i
	}
	fmt.Printf("[%s] StartTime: %d\n", name, i)
}

func (n *Node) findPeer(name string) *Node {
	for _, peer := range n.peers {
		if peer.name == name {
			return peer
		}
	}
	return nil
}

func (n *Node) get(peer *Node, path string) chan []byte {
	c := make(chan []byte)

	go func() {
		if response, err := http.Get("http://" + peer.name + path); err != nil {
			fmt.Println("Error", peer.name+path, " : ", err)
			c <- []byte{}
			return
		} else {
			defer response.Body.Close()
			bodyData, _ := ioutil.ReadAll(response.Body)
			c <- bodyData
			return
		}
	}()

	return c
}

func (n *Node) initPeers(peers ...string) {
	for _, peer := range peers {
		n.peers = append(n.peers, NewPeer(peer))
	}
}

func (n *Node) AddPeers(peers ...*Node) {
	for _, peer := range peers {
		n.peers = append(n.peers, peer)
	}
}

func startupResponse(b []byte) (string, int64) {
	var peerName string
	startTime := int64(-1)
	v := map[string]int64{}
	if err := json.Unmarshal(b, &v); err != nil {
		fmt.Println("Error unmarshalling: ", err)
		return peerName, startTime
	}

	for name, startupTime := range v {
		peerName = name
		startTime = startupTime
	}
	return peerName, startTime
}

func leader(b []byte) string {
	var leaderName string
	if err := json.Unmarshal(b, &leaderName); err != nil {
		fmt.Println("Error unmarshalling leader: ", err)
		return ""
	}
	return leaderName
}
func main() {
	n := NewNode()
	n.Start()
	fmt.Println("Running...")
	time.Sleep(time.Second * 120)

	os.Exit(0)
}
