package dbmodel

import "time"

type Borrow struct {
	BorrowID   string     `bson:"borrow_id"`
	UserID     string     `bson:"user_id"`
	BookID     string     `bson:"book_id"`
	BorrowDate time.Time  `bson:"borrow_date"`
	DueDate    time.Time  `bson:"due_date"`
	ReturnDate *time.Time `bson:"return_date"`
	Status     string     `bson:"status"`
	FineAmount float64    `bson:"fine_amount"`
	DaysLate   int        `bson:"days_late"`
}

// ตัวอย่างข้อมูล
// {
//   "_id": ObjectId,
//   "borrow_id": "BR001",
//   "user_id": "U001",
//   "book_id": "B001",
//   "borrow_date": "2026-02-20T10:00:00Z",
//   "due_date": "2026-02-27T10:00:00Z",
//   "return_date": null,
//   "status": "BORROWED",
//   "fine_amount": 0
// }
// status มีค่า: BORROWED, RETURNED, OVERDUE
