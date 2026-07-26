#pragma once
#include "watch_state.h"
typedef enum { DIAL_BUTTON_PREVIOUS, DIAL_BUTTON_NEXT, DIAL_BUTTON_SELECT } dial_button_t;
void dial_button_press(dial_watch_state_t *state, dial_button_t button);
