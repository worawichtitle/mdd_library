# 📚 Library Management System (Microservices)

โปรเจคนี้เป็นระบบจัดการห้องสมุด (Library Management System) ที่พัฒนาด้วยสถาปัตยกรรม **Microservices** เพื่อแยกการทำงานของแต่ละระบบออกจากกัน และรองรับการขยายตัวของระบบ (Scalability)

---

## 🛠️ เทคโนโลยีที่ใช้

* Microservices Architecture
* RESTful API (GET, POST, PUT, DELETE)
* Database (JSON / MongoDB)
* Postman (สำหรับทดสอบ API)
* Docker (ถ้ามี)

---

## 🧩 โครงสร้างระบบ

ระบบแบ่งออกเป็น 3 Microservices หลัก:

### 👤 User Management Service

จัดการข้อมูลผู้ใช้งาน เช่น

* สร้างผู้ใช้
* แก้ไขข้อมูล
* ลบผู้ใช้
* ดูข้อมูลผู้ใช้

---

### 📖 Book Catalog Service

จัดการข้อมูลหนังสือและเล่มหนังสือ เช่น

* เพิ่ม/แก้ไข/ลบ หนังสือ
* ค้นหาหนังสือ
* จัดการ copy (เล่มจริง)

---

### 🔄 Borrow Service

จัดการการยืม-คืนหนังสือ เช่น

* ยืมหนังสือ
* คืนหนังสือ
* ตรวจสอบสถานะ

---

## 🚀 วิธีการใช้งาน

### 1. เริ่มต้นระบบ

```bash
git clone <your-repo-url>
cd <your-project>
```

(ถ้ามี Docker)

```bash
docker compose up -d --build
```

---

### 2. ทดสอบ API (ผ่าน Postman)

---

## 👤 User Management Service API

* สร้างผู้ใช้
  `POST /users`

```json
{
  "name": "John Doe",
  "email": "john@gmail.com",
  "password": "123456"
}
```

* ดูผู้ใช้ทั้งหมด
  `GET /users`

* ดูผู้ใช้รายบุคคล
  `GET /users/:id`

* แก้ไขผู้ใช้
  `PUT /users/:id`

* ลบผู้ใช้
  `DELETE /users/:id`

---

## 📖 Book Catalog Service API

* ดูหนังสือทั้งหมด
  `GET /books`

* ค้นหาหนังสือ
  `GET /books/search?q=keyword`

* ดู copies ของหนังสือ
  `GET /books/:isbn/copies`

* ตรวจสอบสถานะหนังสือ
  `GET /copies/:barcode/status`

* เพิ่มหนังสือ
  `POST /books`

```json
{
  "title": "Clean Code",
  "author": "Robert C. Martin"
}
```

* เพิ่มเล่มหนังสือ
  `POST /copies`

```json
{
  "isbn": "123456",
  "barcode": "BC001"
}
```

* แก้ไขหนังสือ
  `PUT /books/:isbn`

* แก้ไขเล่มหนังสือ
  `PUT /copies/:barcode`

* ลบหนังสือ
  `DELETE /books/:isbn`

* ลบเล่มหนังสือ
  `DELETE /copies/:barcode`

---

## 🔄 Borrow Service API

* ยืมหนังสือ
  `POST /borrow`

```json
{
  "user_id": "U1",
  "barcode": "BC001"
}
```

* คืนหนังสือ
  `POST /return`

* ตรวจสอบสถานะการยืม
  `GET /borrow/:id`

---

## 🧪 Testing

ระบบถูกทดสอบโดยใช้:

* API Testing (Postman)
* Functional Testing
* Error Handling Testing

### ✔ ตัวอย่าง Test Case

* สร้าง user สำเร็จ
* สร้าง user email ซ้ำ
* ยืมหนังสือสำเร็จ
* ยืมหนังสือที่ถูกยืมแล้ว
* ลบหนังสือที่ยังมี copy

---

## 📂 Database Design

### 👤 User

```json
{
  "user_id": "U1",
  "name": "John Doe",
  "email": "john.doe@gmail.com",
  "password": "asd",
  "status": "ACTIVE",
  "role": "STAFF",
  "created_at": "2026-03-01T10:00:00Z"
}
```

---

### 📖 Book

```json
{
  "isbn": "123456",
  "title": "Clean Code",
  "author": "Robert C. Martin"
}
```

---

### 📦 Copy

```json
{
  "barcode": "BC001",
  "isbn": "123456",
  "status": "AVAILABLE"
}
```

---

## ⚠️ Business Rules

* ไม่สามารถยืมหนังสือที่ถูกยืมไปแล้ว
* ไม่สามารถลบหนังสือ หากยังมี copy อยู่
* email ของ user ต้องไม่ซ้ำ
* barcode ของหนังสือต้องไม่ซ้ำ

---

## 🧹 ปิดระบบ

```bash
docker compose down -v
```

---

## 👨‍💻 ผู้พัฒนา

* 66070031 นางสาวจีรพิชญ์สินี อินผล
* 66070180 นายวรวิชญ์ จุ่นพิจารณ์
* 66070301 นายพีรัช มินเจริญ

