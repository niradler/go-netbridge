package main

// import (
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"sync"

// 	"github.com/go-chi/chi/v5"
// 	"github.com/go-chi/chi/v5/middleware"

// 	"github.com/niradler/go-netbridge/messages"
// 	"github.com/niradler/go-netbridge/tunnel"
// )

// func main() {
// 	r := chi.NewRouter()
// 	r.Use(middleware.Logger)
// 	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
// 		conn, err := tunnel.Create(w, r)
// 		if err != nil {
// 			log.Println("Error upgrading connection:", err)
// 			return
// 		}

// 		defer conn.Close()

// 		var wg sync.WaitGroup
// 		wg.Add(1)

// 		go func() {
// 			defer wg.Done()
// 			for {
// 				msg, err := messages.ReadAndParseMessage(conn)
// 				fmt.Printf("Received message: %+v\n", msg)
// 				if err != nil {
// 					log.Printf("Error reading message: %v", err)
// 					break
// 				}
// 				if msg.Type == messages.MessageType.Response {
// 					fmt.Printf("Received response: %+v\n", msg)
// 				} else {
// 					err = messages.MessageHandler(conn, msg)
// 					if err != nil {
// 						log.Printf("Error handling message: %v", err)
// 					}
// 				}
// 			}
// 		}()

// 		wg.Wait()

// 	})

// 	log.Println("Server started on :8080")
// 	if err := http.ListenAndServe(":8080", r); err != nil {
// 		log.Fatal("ListenAndServe:", err)
// 	}
// }
