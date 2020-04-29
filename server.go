package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alash3al/go-pubsub"
	"github.com/gorilla/websocket"

	"github.com/gin-gonic/gin"
)

var port = "4558"
var broker = pubsub.NewBroker()

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/mainstream", func(c *gin.Context) {
		handleStream(c.Writer, c.Request, "mainstream")
	})

	r.GET("/substream", func(c *gin.Context) {
		handleStream(c.Writer, c.Request, "substream")
	})

	return r
}

func main() {

	mainstreamPipePath := flag.String("m", "", "Mainstream pipe file path")
	substreamPipePath := flag.String("s", "", "Substream pipe file path")

	flag.Parse()

	validateArgs(*mainstreamPipePath, *substreamPipePath)

	go consumeStream(*mainstreamPipePath, "mainstream")
	go consumeStream(*substreamPipePath, "substream")

	r := setupRouter()
	r.Run(":" + port)

}

func validateArgs(mainstreamPipePath string, substreamPipePath string) {

	if(mainstreamPipePath == "" && substreamPipePath == "") {
		fmt.Fprintf(os.Stderr, "Please specify at least one pipe (substream or mainstream)\n")
        os.Exit(1)
	}

	if(mainstreamPipePath != "") {
		_, err := os.Stat(mainstreamPipePath)
		if err != nil {
			fmt.Printf("Please specify a valid mainstream pipe path.\n")
			os.Exit(2)
		}
	}

	if(substreamPipePath != "") {
		_, err := os.Stat(substreamPipePath)
		if err != nil {
			fmt.Printf("Please specify a valid substream pipe path.\n")
			os.Exit(2)
		}
	}
}

func consumeStream(streamPath string, videoChannel string) {
	namedPipe, err := os.OpenFile(streamPath, os.O_RDONLY, os.ModeNamedPipe)

	if err != nil {
		fmt.Println(err)
	}

	bytes := make([]byte, 1024*1024)

	for {
		if broker.Subscribers(videoChannel) == 0 {
			time.Sleep(250 * time.Millisecond)
			continue
		}

		fmt.Println("Reading " + videoChannel)

		bytesRead, _ := namedPipe.Read(bytes)
		broker.Broadcast(bytes[:bytesRead], videoChannel)
	}
}

func handleStream(w http.ResponseWriter, r *http.Request, videoChannel string) {
	stopFlag := false
	conn, _ := wsupgrader.Upgrade(w, r, nil)

	defer conn.Close()

	subscriber, _ := broker.Attach()
	broker.Subscribe(subscriber, videoChannel)

	go listenForCloseWebSocket(conn, &stopFlag)

	channel := subscriber.GetMessages()

	for {
		if msg, ok := <-channel; ok {
			payload := msg.GetPayload().([]byte)

			if stopFlag {
				broker.Unsubscribe(subscriber, videoChannel)
				return
			}

			conn.WriteMessage(2, payload)
		}
	}
}

func listenForCloseWebSocket(conn *websocket.Conn, stopFlag *bool) {
	conn.ReadMessage() // Blocks until socket is closed
	*stopFlag = true
}
