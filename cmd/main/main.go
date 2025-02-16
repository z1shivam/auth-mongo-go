package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type HelloResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Router struct {
	Client *mongo.Client
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading env.")
	}

	client := mongoConnect()
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
		log.Println("DB disconnected")
	}()

	if err := client.Ping(context.Background(), nil); err != nil {
		log.Fatal("Couldn't connect to db")
	}

	log.Println("Successfully connected to MongoDB!")

	router := &Router{Client: client}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", router.HomeHandler)
	r.Post("/register", router.RegisterHandler)

	log.Println("Starting server on " + os.Getenv("LOCALHOST_URL"))
	if os.Getenv("MODE") == "DEV" {
		http.ListenAndServe(os.Getenv("LOCALHOST_URL"), r)
	} else {
		http.ListenAndServe(os.Getenv("PROD_PORT"), r)

	}
}

func mongoConnect() *mongo.Client {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		log.Fatal("Set your 'MONGO_URI' environment variable.")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}
	return client
}

func (router *Router) HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	err := json.NewEncoder(w).Encode(&HelloResponse{
		Success: true,
		Message: "Hello from server",
	})
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

type RegisterUserInput struct {
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

func (router *Router) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")

	var user RegisterUserInput
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if user.Username == "" || user.FullName == "" || user.Password == "" || user.Email == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	collection := router.Client.Database(os.Getenv("MONGO_DB")).Collection("users")
	_, err := collection.InsertOne(context.TODO(), user)
	if err != nil {
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}
