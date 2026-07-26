#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "watch_display.h"
#include "watch_state.h"

void app_main(void) {
  dial_watch_state_t state;
  dial_watch_state_defaults(&state);
  dial_display_init();
  dial_display_render(&state);
  while (true) {
    // TODO: Wi-Fi + HTTPS sync with cached-state fallback.
    vTaskDelay(pdMS_TO_TICKS(30000));
  }
}
