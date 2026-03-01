package main

import (
	"context"
	"log"
	"time"

	dbmodel "borrow-return/db"
)

func main() {

	collection, err := connectMongo()
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	due := now.AddDate(0, 0, 15) // Due in 15 days

	borrow := dbmodel.Borrow{
		BorrowID:   "BR001",
		UserID:     "U001",
		BookID:     "B001",
		BorrowDate: now,
		DueDate:    due,
		ReturnDate: nil,
		Status:     "BORROWED",
		FineAmount: 0,
	}

	_, err = collection.InsertOne(context.Background(), borrow) //เปิดมาแล้วจะยัดข้อมูลนี้เลย เป็นตัวเทส
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Borrow inserted successfully")
}
