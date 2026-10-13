package main

import (
	"bytes"
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectMongo() (*mongo.Database, func()) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "retail"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	cancel()
	return client.Database(dbName), func() { _ = client.Disconnect(context.Background()) }
}

func ensureIndexes(db *mongo.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.Collection("products").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]interface{}{"sku": 1, "active": 1},
	})
	if err != nil {
		log.Printf("index products: %v", err)
	}
	_, err = db.Collection("orders").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]interface{}{"userId": 1, "createdAt": -1},
	})
	if err != nil {
		log.Printf("index orders: %v", err)
	}
}

type Order struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID    string             `bson:"userId" json:"userId"`
	ProductID string             `bson:"productId" json:"productId"`
	Quantity  int                `bson:"quantity" json:"quantity"`
	Status    string             `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

func main() {
	db, closeDB := connectMongo()
	defer closeDB()
	ensureIndexes(db)
	inventoryURL := os.Getenv("INVENTORY_URL")
	if inventoryURL == "" {
		inventoryURL = "http://localhost:8082"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "orders"})
	})
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		cursor, err := db.Collection("orders").Find(r.Context(), bson.M{})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer cursor.Close(r.Context())
		orders := []Order{}
		if err := cursor.All(r.Context(), &orders); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(orders)
	})
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var order Order
		if json.NewDecoder(r.Body).Decode(&order) != nil || order.UserID == "" || order.ProductID == "" || order.Quantity < 1 {
			http.Error(w, "userId, productId and quantity >= 1 required", 400)
			return
		}
		payload, _ := json.Marshal(map[string]int{"delta": -order.Quantity})
		req, _ := http.NewRequestWithContext(r.Context(), "POST", inventoryURL+"/inventory/"+order.ProductID+"/adjust", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "inventory service unavailable", 503)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			http.Error(w, "inventory rejected order", 409)
			return
		}
		order.Status = "created"
		order.CreatedAt = time.Now().UTC()
		result, err := db.Collection("orders").InsertOne(r.Context(), order)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		order.ID = result.InsertedID.(primitive.ObjectID)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(order)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	log.Printf("orders listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
