package main

import "log"

func handleClient(client *Client) {
	defer client.conn.Close()

	// Check the header
	buffer := make([]byte, 2)
	_, err := client.conn.Read(buffer)
	if err != nil {
		log.Print(err)
		return
	}

}
