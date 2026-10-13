package main

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"time"

	"context"
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

type Stock struct {
	SKU      string `bson:"sku" json:"sku"`
	Quantity int    `bson:"quantity" json:"quantity"`
}

func main() {
	db, closeDB := connectMongo()
	defer closeDB()
	ensureIndexes(db)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "inventory"})
	})
	mux.HandleFunc("GET /inventory/{sku}", func(w http.ResponseWriter, r *http.Request) {
		sku := r.PathValue("sku")
		var stock Stock
		err := db.Collection("inventory").FindOne(r.Context(), bson.M{"sku": sku}).Decode(&stock)
		if err == mongo.ErrNoDocuments {
			stock = Stock{SKU: sku, Quantity: 100}
			_, _ = db.Collection("inventory").InsertOne(r.Context(), stock)
		} else if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(stock)
	})
	mux.HandleFunc("POST /inventory/{sku}/adjust", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Delta int `json:"delta"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		sku := r.PathValue("sku")
		filter := bson.M{"sku": sku}
		update := bson.M{"$inc": bson.M{"quantity": body.Delta}, "$setOnInsert": bson.M{"sku": sku}}
		opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
		var stock Stock
		err := db.Collection("inventory").FindOneAndUpdate(r.Context(), filter, update, opts).Decode(&stock)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if stock.Quantity < 0 {
			http.Error(w, "stock cannot be negative", 409)
			return
		}
		json.NewEncoder(w).Encode(stock)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("inventory listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
