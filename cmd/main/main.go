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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
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
	r.Post("/login", router.LoginHandler)

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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
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

	var existingUser RegisterUserInput
	err := collection.FindOne(context.TODO(), bson.D{{Key: "username", Value: user.Username}}).
		Decode(&existingUser)

	if existingUser.Username != "" {
		http.Error(w, "user already exists", http.StatusBadRequest)
		return
	}

	if hashedPassword, err := HashPassword(user.Password); err == nil {
		user.Password = hashedPassword
	}

	if err == mongo.ErrNoDocuments {
		_, err := collection.InsertOne(context.TODO(), user)
		if err != nil {
			http.Error(w, "Failed to register user", http.StatusInternalServerError)
			return
		}
	}

	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

type LoginRes struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    *struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Logintoken string `json:"loginToken"`
	}
}

type UserToBeLoggedIn struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (router *Router) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var gotUser UserToBeLoggedIn
	if err := json.NewDecoder(r.Body).Decode(&gotUser); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	loginRes := &LoginRes{
		Success: false,
		Message: "Can't login user",
		User:    nil,
	}

	var user RegisterUserInput
	collection := router.Client.Database(os.Getenv("MONGO_DB")).Collection("users")
	err := collection.FindOne(context.TODO(), bson.D{{Key: "username", Value: gotUser.Username}}).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	if !VerifyPassword(gotUser.Password, user.Password) {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	loginToken := "some-login-token"

	loginRes.Success = true
	loginRes.Message = "User logged in successfully"
	loginRes.User = &struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Logintoken string `json:"loginToken"`
	}{
		Username:   user.Username,
		Email:      user.Email,
		Logintoken: loginToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Custom-Header", "custom-value")

	http.SetCookie(w, &http.Cookie{
		Name:  "session_token",
		Value: loginToken,
		Path:  "/",
	})

	if err := json.NewEncoder(w).Encode(loginRes); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}
