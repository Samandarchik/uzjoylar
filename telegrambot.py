import requests
from aiogram import Bot, Dispatcher, types
from aiogram.utils import executor

# Bot token
BOT_TOKEN = "8259085925:AAGmO2xe9iIT118bCBPkiOosFofWyPLxQBA"

# API ma'lumotlari
API_URL = "https://agent.monebakeryuz.uz/api/products"
API_TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjEsInJvbGUiOiJhZG1pbiIsImV4cCI6MTc2MTU2NDI1NSwiaWF0IjoxNzYxNDc3ODU1fQ.WY-RU2AA5is3Lqbdi24vynDsE9baw9_15ZzFjcHuFeU"

bot = Bot(token=BOT_TOKEN)
dp = Dispatcher(bot)

@dp.message_handler()
async def handle_message(message: types.Message):
    text = message.text.strip()

    try:
        # xabarni satrlarga bo‘lamiz
        lines = text.split("\n")
        data = {}
        for line in lines:
            if ":" in line:
                key, value = line.split(":", 1)
                data[key.strip()] = value.strip()

        # kerakli maydonlarni olish
        name = data.get("n")
        price = data.get("p")
        category = data.get("c")
        image = data.get("im")
        ingredients = data.get("in")

        if not all([name, price, category, image, ingredients]):
            await message.reply("⚠️ Ma'lumot to‘liq emas. Quyidagicha yuboring:\n\nn:Burger\np:12000\nc:Fast Food\nim:https://example.com/lavash.jpg\nin:Go‘sht, pomidor, sous")
            return

        # API so‘rov yuborish
        payload = {
            "name": name,
            "price": int(price),
            "categoryName": category,
            "imageUrl": image,
            "ingredients": ingredients
        }

        headers = {
            "Authorization": f"Bearer {API_TOKEN}",
            "Content-Type": "application/json"
        }

        response = requests.post(API_URL, json=payload, headers=headers)

        if response.status_code == 200 or response.status_code == 201:
            await message.reply("✅ Mahsulot muvaffaqiyatli qo‘shildi!")
        else:
            await message.reply(f"❌ Xatolik: {response.status_code}\n{response.text}")

    except Exception as e:
        await message.reply(f"❗ Xatolik: {e}")

if __name__ == "__main__":
    executor.start_polling(dp, skip_updates=True)
