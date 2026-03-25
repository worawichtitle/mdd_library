# 📚 ระบบจัดการห้องสมุด (Library Management System)

โปรเจคนี้เป็นระบบจัดการห้องสมุด (Library Management System) ที่พัฒนาด้วยสถาปัตยกรรม **Microservices** โดยใช้ภาษา **Go (Golang)** และเทคโนโลยีที่ทันสมัยเพื่อรองรับการขยายตัว (Scalability) และความทนทาน (Resilience) ของระบบ

---

## 🛠️ เทคนิคและเทคโนโลยีที่ใช้ในโปรเจค

1. **Microservices Architecture**: ระบบถูกแบ่งออกเป็น 3 เซอร์วิสหลักที่ทำงานแยกจากกันอย่างชัดเจน
   - **User Management Service** (Port `8083`): จัดการข้อมูลผู้ใช้งาน การสร้าง แก้ไข ลบ และดูข้อมูลผู้ใช้
   - **Book Catalog Service** (Port `8082`): จัดการข้อมูลหนังสือ เล่มหนังสือ การค้นหา และสถานะเล่มหนังสือ
   - **Borrow-Return Service** (Port `8081`): ดูแลตรรกะการยืม-คืนหนังสือ ตรวจสอบสถานะการยืม และจัดการประวัติการยืม

2. **Go & Gin Framework**:
   - ตัวระบบถูกพัฒนาด้วยภาษา Go ซึ่งโดดเด่นด้วยความเร็ว และการใช้ทรัพยากรน้อย
   - รัน Web Service และจัดการ Routing ของ RESTful API ด้วยโมดูล `gin-gonic/gin`

3. **MongoDB**:
   - เป็น NoSQL Database ที่ใช้จัดการข้อมูลประวัติการยืม
   - มีความยืดหยุ่นสูง เหมาะสำหรับการจัดเก็บข้อมูลอย่างหลากหลาย

4. **JSON Data Storage**:
   - ใช้ไฟล์ JSON สำหรับเก็บข้อมูลหนังสือ เล่มหนังสือ และบัญชีผู้ใช้
   - มีการจัดการข้อมูลอย่างเป็นระบบผ่าน in-memory storage

5. **RabbitMQ (Message Broker)**:
   - ใช้เป็นตัวกลางช่วยให้สามารถสื่อสารแบบ Asynchronous (Event-driven) ระหว่างเซอร์วิส
   - ช่วยลดคอขวดสะสมเมื่อมีคำร้องขอเข้ามาเยอะๆ พร้อมกัน

6. **Service Discovery (Consul)**:
   - นำ **HashiCorp Consul** เพื่อใช้เป็น Service Discovery ช่วยให้ Microservices สามารถค้นหาและเชื่อมต่อกันได้
   - ทำ Health Check ตรวจสอบสถานะของแต่ละเซอร์วิส

7. **Monitoring & Observability (ตรวจวัดสถานะระบบ)**:
   - **Prometheus** (Port `9090`): คอยดึงข้อมูล (Scrape) metrics เช่น Request Count และ Latency ของแต่ละ Services
   - **Grafana** (Port `3000`): ระบบแสดงผล Visualize ข้อมูลให้อยู่ในรูปแบบ Dashboard ที่สวยงามและอัปเดตแบบเรียลไทม์

8. **Docker & Docker Compose**:
   - ควบคุมทุก Environment ให้อยู่ข้างในระบบ Containerization
   - มีการบิวต์ Docker ด้วยแนวคิด **Multi-stage Build** ซึ่งทำให้ขนาดของ Image file สุดท้ายเล็กและปลอดภัย
   - ผสานการสั่งการทุกคอนเทนเนอร์เข้าด้วยกันพร้อมระบบ Network ที่เชื่อมกันผ่านคำสั่ง `docker compose`

---

## 🚀 คู่มือการใช้งาน


### 1. การเริ่มต้นใช้งาน

สิ่งที่จำเป็นต้องติดตั้งก่อนใช้งาน:

- **Go (Golang)**
- **Docker**

### 2. การเปิดและรันระบบ

โคลนโปรเจค และใช้งาน Docker Compose เพื่อเริ่มใช้งาน:

```bash
git clone https://github.com/worawichtitle/mdd_library.git

cd mdd_library
```

สร้าง Docker Containers และเริ่มระบบทั้งหมด:

```bash
docker compose up -d --build
```

### 3. การทดสอบยิง API (ผ่าน Postman, cURL หรือ Thunder Client)

หลังจากระบบเริ่มต้นสำเร็จ สามารถทดสอบยิง API คร่าวๆ ได้ดังนี้:

---

## 🌐 User Management Service API


**สร้างผู้ใช้ใหม่**
```
POST http://localhost:8000/user
```
```json
{
  "name": "John Doe",
  "email": "john@gmail.com",
  "password": "123456"
}
```

**ดูผู้ใช้ทั้งหมด**
```
GET http://localhost:8000/user
```

**ดูผู้ใช้รายบุคคล**
```
GET http://localhost:8000/user/:user_id
```

**ดูผู้ใช้รายบุคคล**
```
GET http://localhost:8000/user/:user_id/verify
```

**แก้ไขข้อมูลผู้ใช้**
```
PUT http://localhost:8000/user/:user_id
```

**ลบผู้ใช้**
```
DELETE http://localhost:8000/user/:user_id
```

---

## 📖 Book Catalog Service API

**เพิ่มหนังสือใหม่**
```
POST http://localhost:8000/books
```
```json
{
  "title": "Clean Code",
  "author": "Robert C. Martin"
}
```

**เพิ่มเล่มหนังสือ (Copy)**
```
POST http://localhost:8000/copies
```
```json
{
  "isbn": "ISBN-1",
  "status": "available",
  "condition": "new"
}
```

**ดูหนังสือทั้งหมด**
```
GET http://localhost:8000/books
```

**ค้นหาหนังสือจากชื่อ**
```
GET http://localhost:8000/books/search?q=
```

**ดู Copies ของหนังสือ**
```
GET http://localhost:8000/books/:isbn/copies
```

**ตรวจสอบสถานะเล่มหนังสือ**
```
GET http://localhost:8000/copies/:barcode/status
```

**แก้ไขข้อมูลหนังสือ**
```
PUT http://localhost:8000/books/:isbn
```

**แก้ไขข้อมูลเล่มหนังสือ**
```
PUT http://localhost:8000/copies/:barcode
```

**ลบหนังสือ** (ต้องไม่มีเล่มหนังสือเหลืออยู่)
```
DELETE http://localhost:8000/books/:isbn
```

**ลบเล่มหนังสือ**
```
DELETE http://localhost:8000/copies/:barcode
```

---

## 🔄 Borrow-Return Service API

**ยืมหนังสือ**
```
POST http://localhost:8000/borrows
```
```json
{
  "user_id": "U1",
  "barcode": "BC-1"
}
```

**คืนหนังสือ**
```
POST http://localhost:8000/return
```
```json
{
  "borrow_id": "BRW-1"
}
```

**ดูประวัติการยืมทั้งหมด**
```
GET http://localhost:8000/borrows
```

**ดูประวัติการยืมทั้งหมดของ user**
```
GET http://localhost:8000/borrows/?user_id=
```

**ดูประวัติการยืมทั้งหมดที่มีสถานะตามกำหนด** (BORROWED, RETURNED)
```
GET http://localhost:8000/borrows/?status=
```

**ดูประวัติการยืมทั้งหมดที่เกินเวลาคืน** (true, false)
```
GET http://localhost:8000/borrows/?overdue=
```

**ดูประวัติการยืมตาม barcode หนังสือ**
```
GET http://localhost:8000/borrows/?barcode=	
```

**ดูประวัติการยืมอันเดียว**
```
GET http://localhost:8000/borrows/:borrow_id
```


---

## 📂 Database Design

### 👤 User Schema

```json
{
  "user_id": "U1",
  "name": "John Doe",
  "email": "john.doe@gmail.com",
  "password": "hashed_password",
  "status": "ACTIVE",
  "role": "STAFF",
  "created_at": "2026-03-01T10:00:00Z"
}
```

---

### 📖 Book Schema

```json
{
  "isbn": "ISBN-1",
  "title": "Clean Code",
  "author": "Robert C. Martin",
  "available_stock": 1,
  "total_stock": 1
}
```

### 📦 Book Copy Schema

```json
{
  "barcode": "BC-1",
  "isbn": "ISBN-1",
  "status": "available",
  "condition": "good"
}
```
---

### 🔖 Borrow Record Schema

```json
{
  "borrow_id": "BR001",
  "user_id": "U1",
  "barcode": "BC001",
  "borrow_date": "2026-03-01T10:00:00Z",
  "due_date": "2026-03-15T10:00:00Z",
  "return_date": null,
  "status": "BORROWED"
}
```

---

## ⚠️ Business Rules

* 📌 ไม่สามารถยืมหนังสือที่ถูกยืมไปแล้ว (สถานะต้องเป็น available)
* 📌 user ยืมหนังสือได้ไม่เกินจำนวนที่ role กำหนด
* 📌 ไม่สามารถลบหนังสือ หากยังมี copy อยู่
* 📌 Email ของ user ต้องไม่ซ้ำ
* 📌 Barcode ของเล่มหนังสือต้องไม่ซ้ำ
* 📌 ผู้ใช้ต้องมีอยู่ในระบบก่อนการยืมหนังสือ

---

## 📊 การตั้งค่า Monitoring (Prometheus & Grafana)

### 4.1 สร้างข้อมูล Traffic

ให้เรียกไปที่ API ต่างๆ ของระบบ เพื่อสร้างข้อมูล Metrics:

```bash
curl -X GET http://localhost:8000/user
curl -X GET http://localhost:8000/books
curl -X GET http://localhost:8000/borrows
```

Refresh API endpoints หลายๆ ครั้ง (แนะนำ 5-10 รอบ) เพื่อสร้าง Traffic

### 4.2 ตั้งค่าการดึงข้อมูลใน Grafana

1. เปิด Browser เข้าไปที่ `http://localhost:3000` (Grafana)
2. ใส่ User: `admin` และ Pass: `admin`
3. ในหน้าแรก ไปที่ **Connections** > เลือก **Data Sources**
4. คลิก **Add data source**
5. เลือก **Prometheus**
6. ในช่อง Prometheus server URL: ใส่ `http://prometheus:9090/`
7. เลื่อนลงมาด้านล่างสุดและกดปุ่ม **Save & test**

### 4.3 สร้าง Dashboard เพื่อดูกราฟ

1. ที่เมนูด้านซ้าย เลือกไอคอน **+** > คลิก **Dashboard**
2. เลือก **Add visualization** และเลือก Data source Prometheus
3. ในช่อง Metrics ลองใส่ Query ต่อไปนี้:

**Request Rate:**
```promql
rate(api_gateway_http_requests_total{path!="/metrics"}[1m])
```

**Average Latency:**
```promql
sum(rate(api_gateway_request_duration_seconds_sum[1m])) / sum(rate(api_gateway_request_duration_seconds_count[1m]))
```

**HTTP Status Rate:**
```promql
sum by(status) (rate(api_gateway_http_requests_total[15m]))
```

**Server Error Rate:**
```promql
sum(rate(api_gateway_http_requests_total{status=~"5.*"}[1m]))
```

4. กดปุ่ม **Run query** เพื่อดูกราฟข้อมูล

---

## 🧰 เครื่องมือช่วยเหลืออื่นๆ

- 📈 **Prometheus UI**: `http://localhost:9090`
- 🐇 **RabbitMQ Management**: `http://localhost:15672` (User: guest / Pass: guest)
- 🟢 **Consul UI**: `http://localhost:8500`
- 🍃 **MongoDB Express** (ถ้าติดตั้ง): `http://localhost:8081`

---

## 🧹 การปิดการทำงานของระบบ

หลังทดสอบเสร็จแล้ว ให้ใช้คำสั่ง:

```bash
docker compose down -v
```

คำสั่งนี้จะ:
- ปิด Containers ทั้งหมด
- ลบ Volumes ทั้งหมด (ข้อมูลในฐานข้อมูลจะถูกลบ)

---

## 👨‍💻 ผู้พัฒนา

* 66070031 นางสาวจีรพิชญ์สินี อินผล
* 66070180 นายวรวิชญ์ จุ่นพิจารณ์
* 66070301 นายพีรัช มินเจริญ

