package main

import (
	"fmt"
	"log"

	"lab5-series-tracker/internal/httpapp"
	"lab5-series-tracker/internal/storage"
)

func main() {

	// Abrir base de datos
	db, err := storage.OpenDB("series.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close() 

	// Crear tabla si no existe
	if err := storage.EnsureSchema(db); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Base de datos lista.")

	server := &httpapp.Server{
		Addr: ":8080",
		DB:   db,
	}

	log.Fatal(server.ListenAndServe())
}
