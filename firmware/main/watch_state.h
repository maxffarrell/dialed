#pragma once

#include <stdbool.h>
#include <stddef.h>

#define DIAL_MAX_TASKS 8
#define DIAL_TITLE_LENGTH 48

typedef struct {
  char id[24];
  char title[DIAL_TITLE_LENGTH];
  int duration_minutes;
  bool completed;
} dial_task_t;

typedef struct {
  char now[16];
  int remaining_minutes;
  dial_task_t tasks[DIAL_MAX_TASKS];
  size_t task_count;
  size_t selected_task;
  unsigned version;
} dial_watch_state_t;

void dial_watch_state_defaults(dial_watch_state_t *state);
void dial_watch_state_order(dial_watch_state_t *state);
bool dial_watch_state_decode(const char *json, dial_watch_state_t *state);
