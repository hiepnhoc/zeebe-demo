package route

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"zeebe/controller"
)

func HandleRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", welcome)
	myRouter.HandleFunc("/zeebe", controller.GetAllInstance).Methods("GET")
	myRouter.HandleFunc("/zeebe/{id}", controller.GetById).Methods("GET")
	myRouter.HandleFunc("/zeebe/{id}", controller.DeleteInstance).Methods("DELETE")
	myRouter.HandleFunc("/zeebe/create", controller.CreateIntance).Methods("POST")
	myRouter.HandleFunc("/zeebe/publishmessage", controller.PublishMessageTest).Methods("POST")

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func welcome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}