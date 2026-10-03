<script lang="ts">
  import { initialNotes, makeWatchState } from '#lib/mock-data.js';
  import { completeTask, formatDuration, orderedTasks } from '#lib/watch.js';
  import type { Note, Task, WatchState } from '#lib/types.js';

  let state: WatchState = makeWatchState();
  let notes: Note[] = initialNotes;
  let showAddTask = false;
  let newTask = '';
  let newDuration = 30;
  let isRecording = false;
  let recorder: MediaRecorder | null = null;
  let audioStream: MediaStream | null = null;
  let audioChunks: Blob[] = [];

  $: tasks = state.tasks;
  $: activeTask = tasks[state.selectedTask] ?? tasks[0];
  $: completed = tasks.filter((task) => task.completed).length;
  $: progress = tasks.length ? Math.round((completed / tasks.length) * 100) : 0;

  function toggleTask(task: Task) {
    state = completeTask(state, task.id);
  }

  function selectTask(index: number) {
    state = { ...state, selectedTask: index };
  }

  function addTask() {
    if (!newTask.trim()) return;
    const task: Task = {
      id: newTask.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
      title: newTask.trim(),
      durationMinutes: Math.max(1, newDuration),
      completed: false
    };
    state = { ...state, tasks: orderedTasks([...state.tasks, task]), version: state.version + 1 };
    newTask = '';
    newDuration = 30;
    showAddTask = false;
  }

  function addNote(note: Note) {
    notes = [note, ...notes];
    state = { ...state, context: note.context, version: state.version + 1 };
  }

  async function toggleVoiceNote() {
    if (isRecording && recorder) {
      recorder.stop();
      return;
    }
    if (!navigator.mediaDevices || !('MediaRecorder' in window)) {
      addNote({ id: `note-${Date.now()}`, createdAt: 'Just now', transcript: 'Voice recording is not supported in this browser.', context: 'Use a browser with microphone support.' });
      return;
    }
    audioStream = await navigator.mediaDevices.getUserMedia({ audio: true });
    audioChunks = [];
    recorder = new MediaRecorder(audioStream);
    recorder.ondataavailable = (event) => audioChunks.push(event.data);
    recorder.onstop = async () => {
      const audio = new Blob(audioChunks, { type: recorder?.mimeType || 'audio/webm' });
      audioStream?.getTracks().forEach((track) => track.stop());
      const form = new FormData();
      form.append('file', audio, 'voice-note.webm');
      const response = await fetch('/api/notes', { method: 'POST', body: form });
      if (response.ok) addNote(await response.json() as Note);
      else addNote({ id: `note-${Date.now()}`, createdAt: 'Just now', transcript: 'Transcription failed. Try recording again.', context: 'The note was not saved.' });
      recorder = null;
    };
    recorder.start();
    isRecording = true;
  }

  function tick(delta: number) {
    state = { ...state, remainingMinutes: Math.max(0, state.remainingMinutes - delta) };
  }

  function moveSelection(delta: number) {
    const next = (state.selectedTask + delta + tasks.length) % tasks.length;
    selectTask(next);
  }
</script>

<div class="app-shell">
  <header class="topbar">
    <a class="brand" href="/" aria-label="Dialed home"><span class="brand-mark">·</span> dialed</a>
    <div class="topbar-meta"><span class="live-dot"></span> device connected <span class="slash">/</span> <span>v0.1</span></div>
  </header>

  <main class="workspace">
    <section class="intro">
      <p class="eyebrow">Personal operating system <span>01</span></p>
      <h1>Keep the work<br /><em>in front of you.</em></h1>
      <p class="lede">Dialed turns a short list of meaningful work into a quiet signal on your wrist.</p>
    </section>

    <section class="studio-grid" aria-label="Watch studio">
      <div class="simulator-panel">
        <div class="panel-heading"><span>Watch preview</span><span class="mono-label">240 × 240 px</span></div>
        <div class="watch-stage">
          <div class="watch-shadow"></div>
          <div class="watch-face" data-testid="watch-face">
            <div class="watch-time">{state.now}</div>
            <div class="watch-rule"></div>
            <div class="remaining">
              <div class="hourglass" aria-hidden="true"><span></span><span></span><i></i></div>
              <div class="remaining-copy"><strong>{state.remainingMinutes}</strong><span>Remaining</span></div>
            </div>
            <div class="watch-rule lower"></div>
            <div class="watch-tasks">
              {#each tasks as task, index}
                <button class:active={index === state.selectedTask} class:done={task.completed} class="watch-task" on:click={() => selectTask(index)}>
                  <span class="watch-checkbox">{task.completed ? '×' : ' '}</span>
                  <span>{task.title}</span>
                </button>
              {/each}
            </div>
          </div>
        </div>
        <div class="simulator-controls" aria-label="Simulator controls">
          <button on:click={() => moveSelection(-1)} aria-label="Previous task">↑</button>
          <button class="complete-control" on:click={() => activeTask && toggleTask(activeTask)} aria-label="Complete selected task">complete</button>
          <button on:click={() => moveSelection(1)} aria-label="Next task">↓</button>
          <button on:click={() => tick(1)} aria-label="Advance one minute">+1 min</button>
        </div>
      </div>

      <div class="dashboard-panel">
        <div class="panel-heading"><span>Today</span><span class="sync-label"><span class="sync-dot"></span> synced {state.syncedAt}</span></div>
        <div class="goal-header">
          <div><span class="goal-kicker">Focus block</span><h2>Make the next<br />thing count.</h2></div>
          <div class="progress-ring" style={`--progress: ${progress}%`}><span>{progress}%</span></div>
        </div>
        <div class="task-list">
          {#each tasks as task, index}
            <div class:task-selected={index === state.selectedTask} class="task-row">
              <button class:checked={task.completed} class="checkbox" on:click={() => toggleTask(task)} aria-label={`Mark ${task.title} complete`}>{task.completed ? '×' : ''}</button>
              <button class:task-title-done={task.completed} class="task-name" on:click={() => selectTask(index)}>{task.title}</button>
              <span class="task-duration">{formatDuration(task.durationMinutes)}</span>
            </div>
          {/each}
        </div>
        {#if showAddTask}
          <form class="add-task" on:submit|preventDefault={addTask}>
            <input bind:value={newTask} placeholder="A small, clear next step" aria-label="Task name" />
            <input bind:value={newDuration} type="number" min="1" aria-label="Duration in minutes" />
            <button type="submit">add</button>
          </form>
        {:else}
          <button class="add-button" on:click={() => (showAddTask = true)}><span>+</span> add task</button>
        {/if}
      </div>
    </section>

    <section class="lower-grid">
      <div class="context-panel">
        <div class="panel-heading"><span>Context memory</span><span class="mono-label">{notes.length} notes</span></div>
        <div class="context-body">
          <div class="context-orbit"><span class="orbit-core"></span><span class="orbit-line one"></span><span class="orbit-line two"></span><span class="orbit-node node-one"></span><span class="orbit-node node-two"></span></div>
          <div><span class="goal-kicker">Current signal</span><p class="context-quote">“{state.context}”</p><button class:recording={isRecording} class="voice-button" on:click={toggleVoiceNote}><span class="mic">{isRecording ? '■' : '◉'}</span>{isRecording ? ' tap to transcribe' : ' add voice note'}</button></div>
        </div>
        <div class="note-list">
          {#each notes as note}
            <article class="note"><span>{note.createdAt}</span><p>{note.transcript}</p><small>{note.context}</small></article>
          {/each}
        </div>
      </div>
      <aside class="principles">
        <p class="eyebrow">Product principles <span>02</span></p>
        <h2>Less noise.<br /><em>More follow-through.</em></h2>
        <ol><li><span>01</span> auto-order by task length</li><li><span>02</span> capture context while it is fresh</li><li><span>03</span> make completion feel physical</li></ol>
      </aside>
    </section>
  </main>

  <footer><span>dialed / goal tracker watch</span><span>built for the work that matters</span></footer>
</div>
