package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("http.DefaultTransport:", http.DefaultTransport)
	resp, err := http.Get("https://api.opendota.com/api/heroes")
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("resp status:", resp.Status)
}
