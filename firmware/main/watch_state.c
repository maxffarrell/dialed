#include "watch_state.h"
#include <string.h>

void dial_watch_state_defaults(dial_watch_state_t *state) {
  memset(state, 0, sizeof(*state));
  strcpy(state->now, "9:41 am");
  state->remaining_minutes = 19;
  state->task_count = 3;
  strcpy(state->tasks[0].id, "website"); strcpy(state->tasks[0].title, "Refine the website"); state->tasks[0].duration_minutes = 42;
  strcpy(state->tasks[1].id, "ab-test"); strcpy(state->tasks[1].title, "Create A/B test"); state->tasks[1].duration_minutes = 28;
  strcpy(state->tasks[2].id, "interview"); strcpy(state->tasks[2].title, "Interview Rian"); state->tasks[2].duration_minutes = 19; state->tasks[2].completed = true;
}

void dial_watch_state_order(dial_watch_state_t *state) {
  for (size_t i = 0; i < state->task_count; i++) for (size_t j = i + 1; j < state->task_count; j++) {
    if (state->tasks[j].duration_minutes > state->tasks[i].duration_minutes) {
      dial_task_t task = state->tasks[i]; state->tasks[i] = state->tasks[j]; state->tasks[j] = task;
    }
  }
}

bool dial_watch_state_decode(const char *json, dial_watch_state_t *state) {
  // The production implementation will use cJSON from the HTTP response.
  // Keep this conservative fallback deterministic for offline boot and host tests.
  if (!json || !state || json[0] != '{') return false;
  dial_watch_state_defaults(state);
  return strstr(json, "remainingMinutes") != NULL;
}
