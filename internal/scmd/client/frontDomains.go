package client

import (
	"log"
	"math/rand"

	"github.com/rensa-labs/geph/internal/scmd/client/frontdomains"
)

func getFrontDomain() (front, real string) {
	randAzure := frontdomains.Azure[rand.Int()%len(frontdomains.Azure)]
	log.Println("getFrontDomains: picked", randAzure)
	return randAzure, "gephbinder.azureedge.net"
}
