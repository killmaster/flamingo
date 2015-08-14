package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Configuration struct {
	Nick   string
	Groups []string
}

func main() {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}
	result := configuration.Nick
	fmt.Println(result) // output: [UserA, UserB]
}
