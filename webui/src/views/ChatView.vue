
<template>
  <div>
    <button @click="$router.push('/conversations')">← Back</button>

    <h2 style="margin-top:10px">{{ title }}</h2>
    <p style="opacity:0.7">Conversation ID: {{ id }}</p>

    <p v-if="err" style="color:red">{{ err }}</p>

    <div v-if="loading">Loading...</div>

    <div v-else class="box">
      <div v-for="m in messages" :key="m.id" class="msg">
        <div>
          <b>{{ m.sender }}</b>
          <span style="margin-left:8px; opacity:0.6">{{ formatTime(m.time) }}</span>
        </div>

        <div v-if="m.type === 'text'">{{ m.text }}</div>
        <div v-else>
          <i>[image]</i>
          <div style="opacity:0.6; font-size:12px">base64 not shown</div>
        </div>

        <div style="font-size:12px; opacity:0.7">
          {{ m.read ? "✓✓" : "✓" }}
        </div>
      </div>
    </div>

    <form class="send" @submit.prevent="sendText">
      <input v-model="text" placeholder="Message..." />
      <button type="submit">Send</button>
    </form>

    <div class="hint">
      Images: later you’ll convert file → base64 and send type=image with media.
    </div>
  </div>
</template>

<script>
import api from "../api";

export default {
  props: ["id"],
  data() {
    return {
      title: "",
      members: [],
      messages: [],
      text: "",
      loading: true,
      err: "",
      timer: null,
    };
  },
  async mounted() {
    await this.load();
    // polling for updates (simple and allowed)
    this.timer = setInterval(this.load, 1500);
  },
  unmounted() {
    if (this.timer) clearInterval(this.timer);
  },
  methods: {
    async load() {
      this.err = "";
      try {
        const res = await api.get(`/conversations/${this.id}`);
        this.title = res.data.title;
        this.members = res.data.members || [];
        // backend returns newest first. we want oldest -> newest in UI:
        this.messages = (res.data.messages || []).slice().reverse();
        this.loading = false;
      } catch (e) {
        this.err = "Failed to load conversation";
        this.loading = false;
      }
    },
    async sendText() {
      const t = this.text.trim();
      if (!t) return;

      try {
        await api.post(`/conversations/${this.id}/messages`, {
          type: "text",
          text: t,
          media: "",
          replyTo: null,
        });
        this.text = "";
        await this.load();
      } catch (e) {
        this.err = "Send failed";
      }
    },
    formatTime(t) {
      try {
        return new Date(t).toLocaleTimeString();
      } catch {
        return "";
      }
    },
  },
};
</script>

<style>
.box { border: 1px solid #ddd; padding: 10px; border-radius: 6px; max-height: 60vh; overflow: auto; }
.msg { border-bottom: 1px solid #eee; padding: 8px 0; }
.send { display: flex; gap: 8px; margin-top: 10px; }
.send input { flex: 1; }
.hint { margin-top: 10px; font-size: 12px; opacity: 0.7; }
</style>
