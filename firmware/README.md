# Dialed ESP-IDF firmware

This is a hardware-ready skeleton for an ESP32 with a GC9A01 240×240 SPI TFT and two buttons. It deliberately keeps board pins in `main/config.h` so the common round display modules can be wired without changing application logic.

Build without hardware:

```sh
idf.py set-target esp32
idf.py build
```

The display implementation is a host-safe stub until the target board's SPI pinout is confirmed. `test/` covers state decoding, task ordering, and button transitions without requiring a connected device.
