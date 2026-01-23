<template>
  <div class="page">
    <div class="topbar">
  <button class="btn" @click="$router.push('/conversations')">← Back</button>

  <div class="titlewrap">
    <h2 class="title">{{ title }}</h2>
    <div class="sub">
      <span class="muted">Conversation:</span> <code>{{ id }}</code>
      <span v-if="isGroup" class="pill">group</span>
    </div>
  </div>

  <button
    v-if="isGroup"
    class="btn"
    @click="$router.push(`/groups/${id}/settings`)"
  >
    Group settings
  </button>

  <button class="btn" @click="load" :disabled="loading">Refresh</button>
</div>


    <p v-if="err" class="err">{{ err }}</p>

    <!-- Reply bar -->
    <div v-if="replyTo" class="bar">
      <div>
        <b>Replying to</b> <code>{{ replyTo }}</code>
        <span class="muted"> — {{ replyPreview(replyTo) }}</span>
      </div>
      <button class="btn btn-small" @click="clearReply">Cancel</button>
    </div>

    <!-- Forward bar -->
    <div v-if="forwarding.messageId" class="bar">
      <div>
        <b>Forward</b> <code>{{ forwarding.messageId }}</code>
      </div>
      <div class="row">
        <select v-model="forwarding.targetConversationId" class="select">
          <option value="">-- choose conversation --</option>
          <option v-for="c in myConversations" :key="c.id" :value="c.id">
            {{ c.title }} {{ c.isGroup ? "(group)" : "(direct)" }}
          </option>
        </select>
        <button
          class="btn btn-small"
          @click="confirmForward"
          :disabled="!forwarding.targetConversationId"
        >
          Forward
        </button>
        <button class="btn btn-small" @click="cancelForward">Cancel</button>
      </div>
    </div>

    <div v-if="loading" class="muted">Loading…</div>

    <div v-else class="chatbox">
      <div v-if="!messages.length" class="muted">No messages yet.</div>

      <div v-for="m in messages" :key="m.id" class="msg" :class="{ mine: isMine(m) }">
        <div class="msg-top">
          <div class="who">
            <b v-if="!isMine(m)">{{ m.sender }}</b>
            <span v-else class="muted">You</span>
            <span class="time">{{ formatTime(m.time) }}</span>
          </div>

          <div class="actions">
            <button class="link" @click="setReply(m.id)">Reply</button>
            <button class="link" @click="startForward(m.id)">Forward</button>
            <button v-if="isMine(m)" class="link danger" @click="deleteMsg(m.id)">Delete</button>
          </div>
        </div>

        <!-- forwarded info -->
        <div v-if="m.forwardedBy" class="meta">
          ↪ forwarded by <b>{{ m.forwardedBy }}</b>
        </div>

        <!-- reply info -->
        <div v-if="m.replyTo" class="meta">
          ↩ reply to <code>{{ m.replyTo }}</code>
          <span class="muted"> — {{ replyPreview(m.replyTo) }}</span>
        </div>

        <!-- body -->
        <div class="body">
          <div v-if="m.type === 'text'" class="text">{{ m.text }}</div>

          <div v-else class="media">
            <img v-if="m.media" :src="m.media" class="img" />
            <span v-else class="muted">[image]</span>
          </div>
        </div>

        <!-- reactions display -->
        <div v-if="m.reactions && m.reactions.length" class="react-list">
          <span v-for="r in m.reactions" :key="r.id" class="react-pill">
            {{ r.emoji }} <span class="muted">({{ r.user }})</span>
          </span>
        </div>

        <!-- reaction buttons -->
        <div class="react-actions">
          <button class="btn btn-tiny" @click="react(m.id, '👍')">👍</button>
          <button class="btn btn-tiny" @click="react(m.id, '😂')">😂</button>
          <button class="btn btn-tiny" @click="react(m.id, '❤️')">❤️</button>
          <button v-if="myReactionId(m)" class="btn btn-tiny" @click="unreact(m.id, myReactionId(m))">
            Remove my reaction
          </button>
        </div>

        <!-- read/delivered -->
        <div v-if="isMine(m)" class="status">
          <span v-if="m.read">✓✓</span>
          <span v-else-if="m.delivered">✓</span>
        </div>
      </div>
    </div>

    <!-- send text -->
    <form class="sendrow" @submit.prevent="sendText">
      <input class="input" v-model="text" placeholder="Message…" />
      <button class="btn" :disabled="sending || !text.trim()">Send</button>
    </form>

    <!-- send image -->
    <div class="sendrow">
      <input type="file" accept="image/*" @change="pickFile" />
      <button class="btn" @click="sendImage" :disabled="sending || !pickedBase64">Send Image</button>
      <button class="btn" @click="clearPicked" :disabled="!pickedBase64">Clear</button>
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
      isGroup: false,

      messages: [],
      loading: true,
      sending: false,
      err: "",

      text: "",
      pickedBase64: "",

      replyTo: null,

      myConversations: [],
      forwarding: { messageId: "", targetConversationId: "" },

      poll: null,
      myUsername: localStorage.getItem("wasa_username") || "",
    };
  },

  async mounted() {
    await this.load();
    await this.loadMyConversations();

    // ✅ polling so delivered/read updates appear automatically
    this.poll = setInterval(() => this.load(), 2000);
  },

  beforeUnmount() {
    if (this.poll) clearInterval(this.poll);
  },

  methods: {
    async load() {
      this.err = "";
      this.loading = true;
      try {
        const res = await api.get(`/conversations/${this.id}`);
        this.title = res.data.title;
        this.members = res.data.members || [];
        this.isGroup = !!res.data.isGroup;

        // backend returns newest-first → UI wants oldest-first (more natural reading)
        this.messages = (res.data.messages || []).slice().reverse();

        this.loading = false;
      } catch (e) {
        this.err = "Failed to load conversation";
        this.loading = false;
      }
    },

    async loadMyConversations() {
      try {
        const res = await api.get("/conversations");
        this.myConversations = res.data.conversations || [];
      } catch {
        // ignore
      }
    },

    formatTime(t) {
      try {
        return new Date(t).toLocaleString();
      } catch {
        return String(t);
      }
    },

    isMine(m) {
      return m.sender === this.myUsername;
    },

    replyPreview(msgId) {
      const m = this.messages.find((x) => x.id === msgId);
      if (!m) return "unknown";
      if (m.type === "image") return "📷 Photo";
      return (m.text || "").slice(0, 60);
    },

    setReply(id) {
      this.replyTo = id;
    },

    clearReply() {
      this.replyTo = null;
    },

    async sendText() {
      if (!this.text.trim()) return;
      this.sending = true;
      this.err = "";
      try {
        await api.post(`/conversations/${this.id}/messages`, {
          type: "text",
          text: this.text,
          replyTo: this.replyTo,
        });
        this.text = "";
        this.replyTo = null;
        await this.load();
      } catch {
        this.err = "Send failed";
      } finally {
        this.sending = false;
      }
    },

    pickFile(e) {
      const f = e.target.files && e.target.files[0];
      if (!f) return;

      const r = new FileReader();
      r.onload = () => {
        this.pickedBase64 = r.result;
      };
      r.readAsDataURL(f);
    },

    clearPicked() {
      this.pickedBase64 = "";
    },

    async sendImage() {
      if (!this.pickedBase64) return;
      this.sending = true;
      this.err = "";
      try {
        await api.post(`/conversations/${this.id}/messages`, {
          type: "image",
          media: this.pickedBase64,
          replyTo: this.replyTo,
        });
        this.clearPicked();
        this.replyTo = null;
        await this.load();
      } catch {
        this.err = "Send failed";
      } finally {
        this.sending = false;
      }
    },

    async deleteMsg(messageId) {
      if (!confirm("Delete this message?")) return;
      this.err = "";
      try {
        await api.delete(`/messages/${messageId}`);
        await this.load();
      } catch {
        this.err = "Delete failed";
      }
    },

    startForward(messageId) {
      this.forwarding.messageId = messageId;
      this.forwarding.targetConversationId = "";
    },

    cancelForward() {
      this.forwarding.messageId = "";
      this.forwarding.targetConversationId = "";
    },

    async confirmForward() {
      if (!this.forwarding.messageId || !this.forwarding.targetConversationId) return;
      this.err = "";
      try {
        await api.post(`/messages/${this.forwarding.messageId}/forward`, {
          conversationId: this.forwarding.targetConversationId,
        });
        this.cancelForward();
        await this.load();
      } catch {
        this.err = "Forward failed";
      }
    },

    myReactionId(m) {
      // If backend allows only 1 reaction per user, this finds it.
      // If backend allows multiple, we still remove only the first.
      const r = (m.reactions || []).find((x) => x.user === this.myUsername);
      return r ? r.id : "";
    },

    async react(messageId, emoji) {
      this.err = "";
      try {
        await api.post(`/messages/${messageId}/comments`, { emoji });
        await this.load();
      } catch {
        this.err = "React failed";
      }
    },

    async unreact(messageId, reactionId) {
      if (!reactionId) return;
      this.err = "";
      try {
        await api.delete(`/messages/${messageId}/comments/${reactionId}`);
        await this.load();
      } catch {
        this.err = "Remove reaction failed";
      }
    },
  },
};
</script>

<style scoped>
.page {
  max-width: 900px;
  margin: 0 auto;
  padding: 14px;
}

.topbar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
}

.titlewrap {
  flex: 1;
  min-width: 0;
}

.title {
  margin: 0;
  font-size: 20px;
}

.sub {
  font-size: 12px;
  opacity: 0.7;
  display: flex;
  gap: 8px;
  align-items: center;
}

.pill {
  background: #eee;
  padding: 2px 8px;
  border-radius: 999px;
}

.err {
  color: #b00020;
  margin: 8px 0;
}

.muted {
  opacity: 0.65;
}

.bar {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  padding: 10px;
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 10px;
  margin: 8px 0;
}

.row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.chatbox {
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  padding: 10px;
  min-height: 220px;
  max-height: 60vh;
  overflow: auto;
  background: white;
}

.msg {
  border-bottom: 1px solid #f0f0f0;
  padding: 10px 6px;
}

.msg:last-child {
  border-bottom: none;
}

.msg.mine {
  background: #fbfbfb;
}

.msg-top {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  margin-bottom: 4px;
}

.who {
  display: flex;
  gap: 10px;
  align-items: center;
  min-width: 0;
}

.time {
  font-size: 12px;
  opacity: 0.6;
}

.actions {
  display: flex;
  gap: 10px;
}

.link {
  border: none;
  background: transparent;
  padding: 0;
  cursor: pointer;
  text-decoration: underline;
  font-size: 12px;
  opacity: 0.85;
}

.link:hover {
  opacity: 1;
}

.danger {
  color: #b00020;
}

.meta {
  font-size: 12px;
  opacity: 0.75;
  margin: 4px 0;
}

.body {
  margin: 6px 0;
}

.text {
  white-space: pre-wrap;
}

.media .img {
  max-width: 320px;
  border-radius: 10px;
  border: 1px solid #eee;
}

.react-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 6px;
}

.react-pill {
  background: #f4f4f4;
  border-radius: 999px;
  padding: 3px 10px;
  font-size: 12px;
}

.react-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 8px;
}

.status {
  text-align: right;
  font-size: 12px;
  opacity: 0.6;
  margin-top: 6px;
}

.sendrow {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-top: 10px;
}

.input {
  flex: 1;
  padding: 10px;
  border-radius: 10px;
  border: 1px solid #ddd;
}

.select {
  padding: 8px;
  border-radius: 10px;
  border: 1px solid #ddd;
}

.btn {
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid #ddd;
  background: #fff;
  cursor: pointer;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-small {
  padding: 6px 10px;
  font-size: 12px;
}

.btn-tiny {
  padding: 4px 8px;
  font-size: 12px;
}
</style>
