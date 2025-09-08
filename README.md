# GoalDial Watch

A stateless, open-source ESP32 watch with a round monochrome OLED that streams **raw speech** to the cloud when you finish a task. The server performs **STT** (e.g., NVIDIA Parakeet-TDT-0.6B-v2), stores **daily Markdown logs**, and generates **automatic hierarchical summaries** (daily / weekly / monthly).

## Highlights
- **No on-device storage.** All logs live in the cloud as Markdown files (`YYYY-MM-DD.md`).
- **One button UX.** Long press: record & upload (16 kHz L16 PCM via HTTP chunked POST).
- **Clean watchface.** Progress ring + time remaining + status, in **Geist Mono** (OFL).
- **Simple contract.** `POST /ingest?device_id=…&task_id=…&ts=…` with `audio/L16`.

## Hardware
- ESP32 Dev Module (WROOM or S3)
- Round 128×128 mono OLED (SH1107/SSD1327 via I²C)
- I²S MEMS mic (e.g., INMP441 / ICS-43434)
- 1× momentary button

_Default pins (change in code):_
- I²C: SDA=21, SCL=22
- I²S: BCLK=27, WS=25, DIN=26
- Button: GPIO0

## Cloud contract (device → server)
