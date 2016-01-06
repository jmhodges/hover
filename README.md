Hover
=====

Hover is a client library for the unofficial, unsupported [Hover](https://www.hover.com/) DNS API.

[![GoDoc](https://godoc.org/github.com/jmhodges/hover?status.svg)](http://godoc.org/github.com/jmhodges/hover)

It provides the basic ability to add, remove, and list DNS records and other
domain and user information.

Example
-------

A small example of how to use hover to add a DNS record to a domain your user
has:

    package main
    
    import (
    	"log"
    	"net/http"
    	"os"
    	"time"
    
    	"github.com/jmhodges/hover"
    	"golang.org/x/net/context"
    )
    
    func main() {
    	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
    	cook, err := hover.Login(ctx, http.DefaultClient, os.Getenv("HOVER_USERNAME"), os.Getenv("HOVER_PASSWORD"))
    	if err != nil {
    		log.Fatalf("unable to login to Hover (be sure to set HOVER_USERNAME and HOVER_PASSWORD env vars): %s", err)
    	}
    
    	hc := hover.NewClient(http.DefaultClient, cook)
    	ctx, _ = context.WithTimeout(context.Background(), 1*time.Second)
    	doms, err := hc.DNS(ctx)
    	if err != nil {
    		log.Fatalf("unable to get DNS information for logged-in hover user: %s", err)
    	}
    	domain := "example.com"
    	var example *hover.DNSDomain
    	for _, d := range doms {
    		if d.DomainName == domain {
    			example = d
    			break
    		}
    	}
    	if example == nil {
    		log.Fatalf("unable to find domain %#v in logged-in user's domains", domain)
    	}
    
    	ctx, _ = context.WithTimeout(context.Background(), 1*time.Second)
    	ndr := &hover.NewDNSRecord{
    		Type:    hover.TXT,
    		Name:    "_acme-challenge.example.com",
    		Content: "coolstuff",
    		TTL:     900 * time.Second,
    	}
    	err = hc.AddDNSRecord(ctx, example.ID, ndr)
    	if err != nil {
    		log.Fatalf("unable to add DNS record to %s: %s", domain, err)
    	}
    }
