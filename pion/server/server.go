package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/randutil"
)

//nolint
var (
	isControlling                 bool
	iceAgent                      *ice.Agent
	remoteAuthChannel             chan string
	localHTTPPort, remoteHTTPPort int
)

// HTTP Listener to get ICE Credentials from remote Peer
func remoteAuth(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic(err)
	}

	remoteAuthChannel <- r.PostForm["ufrag"][0]
	remoteAuthChannel <- r.PostForm["pwd"][0]
}

// HTTP Listener to get ICE Candidate from remote Peer
func remoteCandidate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic(err)
	}

	c, err := ice.UnmarshalCandidate(r.PostForm["candidate"][0])
	if err != nil {
		panic(err)
	}
	fmt.Printf("candidate=%s\n", r.PostForm["candidate"])
	if err := iceAgent.AddRemoteCandidate(c); err != nil {
		panic(err)
	}
}

func runHTTPServer() {

}

func createIceAgent() {
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	})
	if err != nil {
		panic(err)
	}

	// When we have gathered a new ICE Candidate send it to the remote peer
	if err = iceAgent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}

		_, err = http.PostForm(fmt.Sprintf("http://localhost:%d/remoteCandidate", remoteHTTPPort), //nolint
			url.Values{
				"candidate": {c.Marshal()},
			})
		if err != nil {
			panic(err)
		}
	}); err != nil {
		panic(err)
	}

	// When ICE Connection state has change print to stdout
	if err = iceAgent.OnConnectionStateChange(func(c ice.ConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", c.String())
	}); err != nil {
		panic(err)
	}

	// Get the local auth details and send to remote peer
	localUfrag, localPwd, err := iceAgent.GetLocalUserCredentials()
	if err != nil {
		panic(err)
	}

	_, err = http.PostForm(fmt.Sprintf("http://localhost:%d/remoteAuth", remoteHTTPPort), //nolint
		url.Values{
			"ufrag": {localUfrag},
			"pwd":   {localPwd},
		})
	if err != nil {
		panic(err)
	}

	remoteUfrag := <-remoteAuthChannel
	remotePwd := <-remoteAuthChannel

	if err = iceAgent.GatherCandidates(); err != nil {
		panic(err)
	}

	fmt.Printf("%s %s", remoteUfrag, remotePwd)
}

func main() { //nolint
	var (
		err  error
		conn *ice.Conn
	)
	remoteAuthChannel = make(chan string, 3)

	flag.BoolVar(&isControlling, "controlling", false, "is ICE Agent controlling")
	flag.Parse()

	if isControlling {
		localHTTPPort = 9000
		remoteHTTPPort = 9001
	} else {
		localHTTPPort = 9001
		remoteHTTPPort = 9000
	}

	http.HandleFunc("/remoteAuth", remoteAuth)
	http.HandleFunc("/remoteCandidate", remoteCandidate)
	go func() {
		if err = http.ListenAndServe(fmt.Sprintf(":%d", localHTTPPort), nil); err != nil {
			panic(err)
		}
	}()
	//
	//if isControlling {
	//	fmt.Println("Local Agent is controlling")
	//} else {
	//	fmt.Println("Local Agent is controlled")
	//}
	//fmt.Print("Press 'Enter' when both processes have started")
	//if _, err = bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
	//	panic(err)
	//}

	fmt.Printf("Gather candidate finished.")

	//conn, err = iceAgent.Dial(context.TODO(), remoteUfrag, remotePwd)
	if err != nil {
		panic(err)
	}

	// Send messages in a loop to the remote peer
	go func() {
		for {
			time.Sleep(time.Second * 3)

			val, err := randutil.GenerateCryptoRandomString(15,
				"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
			if err != nil {
				panic(err)
			}
			if _, err = conn.Write([]byte(val)); err != nil {
				panic(err)
			}

			fmt.Printf("Sent: '%s'\n", val)
		}
	}()

	// Receive messages in a loop from the remote peer
	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Received: '%s'\n", string(buf[:n]))
	}
}
