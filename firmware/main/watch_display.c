#include "watch_display.h"
#include "esp_log.h"

static const char *TAG = "dial-display";

void dial_display_init(void) {
  // TODO: initialize GC9A01 SPI commands once the board pinout is confirmed.
  ESP_LOGI(TAG, "GC9A01 display ready (240x240)");
}

void dial_display_render(const dial_watch_state_t *state) {
  if (!state) return;
  ESP_LOGI(TAG, "%s | %d Remaining | selected=%d", state->now, state->remaining_minutes, (int)state->selected_task);
  // TODO: draw circular mask, Geist Mono subset, hourglass and task rows.
}
