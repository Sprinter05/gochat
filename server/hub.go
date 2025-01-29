package main

type User struct {
	name string
	cl   Client
}

type Hub struct {
	users map[string]User
}
