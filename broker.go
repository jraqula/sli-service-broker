package main

import (
	"broker/sli_broker"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	fmt.Println("ak")

	rtr := mux.NewRouter()

	logger := lager.NewLogger("broker")

	brokerapi.AttachRoutes(rtr, sli_broker.NewSliBroker(logger), logger)

	http.ListenAndServe("0.0.0.0:8080", rtr)
}
