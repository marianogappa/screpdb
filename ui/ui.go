package ui

import (
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/gorilla/websocket"
	"github.com/marianogappa/screpdb/internal/models"
)

type UI struct {
	ch chan *models.ReplayData
}

func NewUI(ch chan *models.ReplayData) *UI {
	return &UI{ch}
}

func (u *UI) Start() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs(u.ch))
	fmt.Println("Server listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

var (
	homeTempl = template.Must(template.New("").Parse(homeHTML))
	upgrader  = websocket.Upgrader{}
)

func reader(ws *websocket.Conn) {
	defer ws.Close()
	for {
		_, p, err := ws.ReadMessage()
		fmt.Println("received message:", string(p))
		if err != nil {
			break
		}
	}
}

func writer(ws *websocket.Conn, ch chan *models.ReplayData) {
	for replayData := range ch {
		ws.WriteMessage(websocket.TextMessage, []byte(analyze(replayData)))
	}
}

func serveWs(ch chan *models.ReplayData) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}
		go writer(ws, ch)
		reader(ws)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var v = struct {
		Host string
		Data string
	}{
		r.Host,
		"Waiting for games...",
	}
	homeTempl.Execute(w, &v)
}

const homeHTML = `<!DOCTYPE html>
<html lang="en">
    <head>
        <title>WebSocket Example</title>
		<style type="text/css">
		</style>
    </head>
    <body>
        <pre id="data">{{.Data}}</pre>
        <script type="text/javascript">
            (function() {
                var data = document.getElementById("data");
                var conn = new WebSocket("ws://{{.Host}}/ws");
                conn.onclose = function(evt) {
                    data.textContent = 'Connection closed';
                }
                conn.onmessage = function(evt) {
                    console.log('message received');
                    data.textContent = evt.data;
                }
				window.setInterval(() => { conn.send(JSON.stringify({hello: "world"})) }, 5000)
            })();
        </script>
    </body>
</html>
`
