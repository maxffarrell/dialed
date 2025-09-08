/*
  GoalDial Watch (ESP32 + Round Mono OLED)
  - Stateless device: no persistent storage
  - Long-press button: capture I2S audio and stream as HTTP chunked 16 kHz L16 PCM to your /ingest endpoint
  - Display: round progress ring + time remaining + status line
  - Fonts: Geist Mono via U8g2 custom bitmap (fallback to built-ins if headers not present)

  Wiring (adjust as needed):
  - I2C OLED: SDA=21, SCL=22
  - I2S MEMS mic: BCLK=27, WS/LRCLK=25, DIN=26
  - Button: GPIO0 (pull-up)
*/

#include <Arduino.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <U8g2lib.h>
#include "driver/i2s.h"

// --- Secrets (keep your real OPEN keys out of the repo) ---
#include "../include/secrets.h"

// --- Display (round 128x128 SH1107 via I2C). Swap constructor if your controller differs ---
U8G2_SH1107_128X128_F_HW_I2C u8g2(U8G2_R0, /* reset=*/ U8X8_PIN_NONE);

// --- Try to use custom Geist Mono if present; otherwise fall back to built-ins ---
#if __has_include("fonts/geistmono_32_tf.h")
  #include "fonts/geistmono_32_tf.h"
  #define FONT_BIG u8g2_font_geistmono32_tf
#else
  #define FONT_BIG u8g2_font_logisoso32_tf
#endif

#if __has_include("fonts/geistmono_12_tf.h")
  #include "fonts/geistmono_12_tf.h"
  #define FONT_SMALL u8g2_font_geistmono12_tf
#else
  #define FONT_SMALL u8g2_font_6x12_tf
#endif

// --- Pins ---
static const int BTN_PIN = 0;   // momentary button
static const int I2S_WS  = 25;  // LRCLK/WS
static const int I2S_SD  = 26;  // DOUT from mic
static const int I2S_SCK = 27;  // BCLK

// --- Audio config ---
static const int SAMPLE_RATE       = 16000; // 16 kHz mono
static const int I2S_NUM_ID        = I2S_NUM_0;
static const size_t CHUNK_SAMPLES  = 1024;  // ~64 ms per chunk
static const size_t IN_BYTE_DEPTH  = 4;     // many MEMS mics output 24-bit in 32-bit frames
static const size_t OUT_BYTE_DEPTH = 2;     // we stream 16-bit (L16)

// --- App state ---
struct {
  bool capturing = false;
  bool wifiOK = false;
  uint32_t sessionSeconds = 25 * 60;
  uint32_t remaining      = 25 * 60;
  unsigned long lastTick  = 0;
  String status = "Boot…";
} app;

// --- UI ---
static inline void drawFace(const char* statusOverride = nullptr) {
  u8g2.clearBuffer();
  const int cx=64, cy=64, r=61;
  u8g2.drawCircle(cx, cy, r, U8G2_DRAW_ALL);

  float pct = app.remaining / float(app.sessionSeconds);
  pct = constrain(pct, 0.0f, 1.0f);
  const int steps = 60;
  const int lit   = int(pct * steps + 0.5f);
  for (int i=0; i<steps; i++) {
    float th = (i/float(steps))*2*PI - PI/2;
    int x1=cx+cos(th)*(r-2), y1=cy+sin(th)*(r-2);
    int x2=cx+cos(th)*(r-8), y2=cy+sin(th)*(r-8);
    if (i <= lit) u8g2.drawLine(x1,y1,x2,y2);
  }

  // big time
  char buf[16];
  uint32_t t=app.remaining; uint16_t mm=t/60, ss=t%60;
  snprintf(buf, sizeof(buf), "%02u:%02u", mm, ss);
  u8g2.setFont(FONT_BIG);
  int w=u8g2.getStrWidth(buf);
  u8g2.drawStr(cx - w/2, cy + 12, buf);

  // footer status
  const String s = statusOverride ? String(statusOverride)
                                  : (app.capturing ? "Recording…" : (app.wifiOK ? "Ready" : "WiFi…"));
  u8g2.setFont(FONT_SMALL);
  int sw = u8g2.getStrWidth(s.c_str());
  u8g2.drawStr(cx - sw/2, 118, s.c_str());

  u8g2.sendBuffer();
}

// --- Wi-Fi ---
static void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 15000) {
    drawFace("WiFi connecting…");
    delay(250);
  }
  app.wifiOK = (WiFi.status() == WL_CONNECTED);
}

// --- I2S init ---
static esp_err_t initI2S() {
  i2s_config_t cfg = {
    .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_RX),
    .sample_rate = SAMPLE_RATE,
    .bits_per_sample = I2S_BITS_PER_SAMPLE_32BIT,
    .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
    .communication_format = I2S_COMM_FORMAT_STAND_I2S,
    .intr_alloc_flags = 0,
    .dma_buf_count = 8,
    .dma_buf_len = 512,
    .use_apll = false,
    .tx_desc_auto_clear = false,
    .fixed_mclk = 0
  };
  i2s_pin_config_t pins = {
    .bck_io_num = I2S_SCK,
    .ws_io_num = I2S_WS,
    .data_out_num = I2S_PIN_NO_CHANGE,
    .data_in_num = I2S_SD
  };
  ESP_ERROR_CHECK(i2s_driver_install(I2S_NUM_ID, &cfg, 0, nullptr));
  return i2s_set_pin(I2S_NUM_ID, &pins);
}

// --- Convert 32-bit (24-bit valid) samples -> 16-bit L16 ---
static inline void convert32to16(const int32_t* src, int16_t* dst, size_t n) {
  for (size_t i=0;i<n;i++) dst[i] = (int16_t)(src[i] >> 14); // gain tweak as needed
}

// --- Chunked HTTP helpers (manual, to minimize RAM) ---
static bool sendChunk(WiFiClient& client, const uint8_t* data, size_t len) {
  char hdr[16];
  int n = snprintf(hdr, sizeof(hdr), "%X\r\n", (unsigned)len);
  if (client.write((const uint8_t*)hdr, n) != n) return false;
  if (len && client.write(data, len) != (int)len) return false;
  if (client.write((const uint8_t*)"\r\n", 2) != 2) return false;
  return true;
}

static bool beginChunkedPOST(WiFiClient& client, const char* pathAndQuery) {
  // Parse API_BASE_URL (e.g., https://api.example.com/ingest)
  String full = String(API_BASE_URL);
  bool https = full.startsWith("https://");
  String hostport = full; hostport.replace("https://",""); hostport.replace("http://","");
  int slash = hostport.indexOf('/');
  String host = (slash>0) ? hostport.substring(0, slash) : hostport;
  String basePath = (slash>0) ? hostport.substring(slash) : String("/");
  String path = basePath + pathAndQuery;

  String hostname = host;
  int port = https ? 443 : 80;
  int colon = host.indexOf(':');
  if (colon>0) { port = host.substring(colon+1).toInt(); hostname = host.substring(0, colon); }

  if (https) ((WiFiClientSecure*)&client)->setInsecure(); // TODO: pin cert in prod

  if (!client.connect(hostname.c_str(), port)) return false;

  String req =
    String("POST ") + path + " HTTP/1.1\r\n" +
    "Host: " + hostname + "\r\n" +
    "User-Agent: esp32-goaldial/0.1\r\n" +
    "Authorization: Bearer " + String(API_BEARER) + "\r\n" +
    "X-Device-ID: " + String(DEVICE_ID) + "\r\n" +
    "Content-Type: audio/L16; rate=16000; channels=1\r\n" +
    "Transfer-Encoding: chunked\r\n" +
    "Connection: close\r\n\r\n";
  client.print(req);
  return true;
}

static bool finishChunkedPOST(WiFiClient& client, String& statusLine) {
  if (!sendChunk(client, nullptr, 0)) return false; // terminating chunk
  unsigned long t0=millis();
  while (client.connected() && millis()-t0<5000) {
    if (client.available()) {
      statusLine = client.readStringUntil('\n');
      break;
    }
  }
  while (client.connected() && client.available()) client.read(); // drain
  client.stop();
  return statusLine.startsWith("HTTP/1.1 200");
}

static bool streamAudio(uint32_t maxMs, const char* query) {
  WiFiClientSecure client;
  if (!beginChunkedPOST(client, query)) { app.status = "HTTP connect fail"; return false; }

  const size_t inBytes  = CHUNK_SAMPLES * IN_BYTE_DEPTH;
  const size_t outBytes = CHUNK_SAMPLES * OUT_BYTE_DEPTH;
  int32_t* inBuf  = (int32_t*)malloc(inBytes);
  int16_t* outBuf = (int16_t*)malloc(outBytes);
  if (!inBuf || !outBuf) { if (inBuf) free(inBuf); if (outBuf) free(outBuf); app.status="No RAM"; return false; }

  unsigned long tStart = millis();
  while (millis() - tStart < maxMs && app.capturing) {
    size_t br = 0;
    if (i2s_read(I2S_NUM_ID, (void*)inBuf, inBytes, &br, portMAX_DELAY) != ESP_OK || br==0) continue;
    size_t samples = br / IN_BYTE_DEPTH;
    convert32to16(inBuf, outBuf, samples);
    if (!sendChunk(client, (uint8_t*)outBuf, samples * OUT_BYTE_DEPTH)) { app.status="Chunk send fail"; free(inBuf); free(outBuf); client.stop(); return false; }
    drawFace("Recording…");
    delay(1);
  }
  free(inBuf); free(outBuf);
  String status;
  bool ok = finishChunkedPOST(client, status);
  app.status = ok ? "Uploaded" : "HTTP end fail";
  return ok;
}

void setup() {
  pinMode(BTN_PIN, INPUT_PULLUP);
  Serial.begin(115200);

  u8g2.begin();
  u8g2.setContrast(255);
  drawFace("Boot…");

  connectWiFi();
  drawFace(app.wifiOK ? "WiFi OK" : "WiFi fail");

  if (initI2S()!=ESP_OK) {
    app.status = "I2S init fail";
    drawFace(app.status.c_str());
  }

  app.lastTick = millis();
}

void loop() {
  // 1 Hz timer UI
  unsigned long now = millis();
  if (now - app.lastTick >= 1000) {
    app.lastTick = now;
    if (app.remaining>0) app.remaining--;
    drawFace(app.status.c_str());
  }

  // button: short press = toggle status, long press = record & upload
  static bool last = HIGH; static unsigned long tDown=0;
  bool cur = digitalRead(BTN_PIN);
  if (last==HIGH && cur==LOW) tDown = now;
  if (last==LOW && cur==HIGH) {
    unsigned long dur = now - tDown;
    if (dur >= 800) {
      if (WiFi.status()!=WL_CONNECTED) connectWiFi();
      if (WiFi.status()==WL_CONNECTED) {
        app.capturing = true;
        app.status = "Recording…";
        String q = String("?device_id=") + DEVICE_ID + "&task_id=current&ts=" + String((uint32_t)time(nullptr));
        streamAudio(20000, q.c_str()); // up to 20s
        app.capturing = false;
      } else {
        app.status = "No WiFi";
      }
    } else {
      app.status = app.status.length() ? "" : "Ready";
    }
  }
  last = cur;

  delay(5);
}
