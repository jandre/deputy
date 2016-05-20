package main

import (
	"os"
)

var users = make(map[string]UserInformation)

type UserInformation struct {
	Name        string
	Username    string
	PhoneNumber string
}

// Setup sets up the users.
//
// This is hard-coded for now. :)
func Setup() {
	users["jandre"] = UserInformation{
		"Jen",
		"jandre",
		os.Getenv("JEN_PHONE"),
	}
}
