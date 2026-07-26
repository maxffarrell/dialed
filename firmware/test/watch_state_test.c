#include <assert.h>
#include <string.h>
#include "../main/watch_state.h"
#include "../main/watch_input.h"

int main(void) {
  dial_watch_state_t state;
  dial_watch_state_defaults(&state);
  assert(state.task_count == 3);
  assert(strcmp(state.tasks[0].title, "Refine the website") == 0);
  dial_button_press(&state, DIAL_BUTTON_NEXT);
  assert(state.selected_task == 1);
  dial_button_press(&state, DIAL_BUTTON_SELECT);
  assert(state.tasks[1].completed == true);
  dial_button_press(&state, DIAL_BUTTON_PREVIOUS);
  assert(state.selected_task == 0);
  return 0;
}
