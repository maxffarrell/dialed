#include "watch_input.h"

void dial_button_press(dial_watch_state_t *state, dial_button_t button) {
  if (!state || state->task_count == 0) return;
  if (button == DIAL_BUTTON_PREVIOUS) state->selected_task = (state->selected_task + state->task_count - 1) % state->task_count;
  if (button == DIAL_BUTTON_NEXT) state->selected_task = (state->selected_task + 1) % state->task_count;
  if (button == DIAL_BUTTON_SELECT) state->tasks[state->selected_task].completed = !state->tasks[state->selected_task].completed;
}
