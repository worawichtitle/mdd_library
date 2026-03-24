# mdd_library
โปรเจคห้องสมุด

วิธีดูข้อมูล MongoDB
1. docker compose up --build         <สร้าง docker>
2. docker ps        <เอาไว้ดูชื่อ docker>
3. docker exec -it {ชื่อ docker} mongosh
4. use library
5. show collections                  <ดูว่ามี borrows ไหม>
6. db.borrows.find().pretty()        <ดูข้อมูลว่ามาแล้ว>

วิธีใช้ user-management
    GET /user                       <ดู user ทั้งหมด>
    GET /user/:id                   <ดู user รายคนตาม id>
    GET /user/:id/verify            <ดูว่า user คนนั้นมีสิทธิ์ยืมไหม>
    POST /user                      <เพิ่มบัญชีผู้ใช้ใหม่ {
                                        "name": "",         (Must: ชื่อบัญชีผู้ใช้)
                                        "email": "",        (Must: อีเมลผู้ใช้ ห้ามซ้ำคนอื่น)
                                        "password": "",     (Must: รหัสผ่าน จะ hash ให้อัตโนมัติ)
                                        "role": ""          (Optional: บทบาท ตัวใหญ่หมด Default: "GUEST")
                                    }>
    PUT /user/:id                 <แก้ไขข้อมูลบัญชีตาม id>
    DELETE /user/:id                <ลบบัญชีผู้ใช้ id นั้น>

วิธีใช้ book-catalog
    GET /books                      <ดูเรื่องหนังสือทั้งหมดที่มี>
    GET /books/:isbn/copies         <ดูว่าหนังสือเรื่องนี้จากรหัส isbn มีเล่มไหนบ้างให้ยืม>
    GET /books/?search=""           <ดูว่ามีชื่อหนังสือตรงกับที่ search ไหม>
    GET /copies/:barcode/status     <ดูว่าหนังสือจาก barcode ว่างให้ยืมรึเปล่า>
    POST /books                     <เพิ่มหนังสือเรื่องใหม่ {
                                        "title": "",        (Must: ชื่อเรื่องของหนังสือ)
                                        "author": ""        (Must: ผู้แต่งหนังสือ)
                                    }>
    POST /copies                    <เพิ่มหนังสือเล่มใหม่ในแต่ละเรื่อง {
                                        "isbn": "",         (Must: รหัสisbnของหนังสือเรื่องที่จะเพิ่ม)
                                        "status": "",       (Optional: สถานะหนังสือ Default: "available")
                                        "condition": ""     (Optional: สภาพหนังสือ Default: "new")
                                    }>
    PUT /books/:isbn                <แก้ข้อมูลหนังสือเรื่องนั้นตาม isbn>
    PUT /copies/:barcode            <แก้ข้อมูลหนังสือเล่มนั้นตาม barcode>
    DELETE /books/:isbn             <ลบเรื่องหนังสือตาม isbn จะต้องไม่มีหนังสือเรื่องนี้เหลืออยู่>
    DELETE /copies/:barcode         <ลบหนังสืมเล่มนั้นตาม barcode>

วิธีใช้ borrow-return
    GET /borrows                    <ดูว่ามีใครยืมหนังสือเล่มไหนบ้าง>