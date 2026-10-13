package main

import (
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type Product struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	SKU    string             `bson:"sku" json:"sku"`
	Name   string             `bson:"name" json:"name"`
	Price  float64            `bson:"price" json:"price"`
	Active bool               `bson:"active" json:"active"`
}

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

func main() {
	db, closeDB := connectMongo()
	defer closeDB()
	ensureIndexes(db)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "catalog"})
	})
	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		cursor, err := db.Collection("products").Find(ctx, bson.M{"active": true})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer cursor.Close(ctx)
		products := []Product{}
		if err := cursor.All(ctx, &products); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(products)
	})
	mux.HandleFunc("POST /products", func(w http.ResponseWriter, r *http.Request) {
		var p Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.SKU == "" || p.Name == "" || p.Price < 0 {
			http.Error(w, "valid sku, name and non-negative price required", 400)
			return
		}
		p.Active = true
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		result, err := db.Collection("products").InsertOne(ctx, p)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		p.ID = result.InsertedID.(primitive.ObjectID)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("catalog listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
