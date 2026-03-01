# mdd_library
โปรเจคห้องสมุด

วิธีดูข้อมูล MongoDB
1. docker compose up --build        <สร้าง docker>
2. docker ps        <เอาไว้ดูชื่อ docker>
3. docker exec -it {ชื่อ docker} mongosh
4. use library
5. show collections        <ดูว่ามี borrows ไหม>
6. db.borrows.find().pretty()        <ดูข้อมูลว่ามาแล้ว>
