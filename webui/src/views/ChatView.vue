<template>
  <div>
    <button @click="$router.push('/conversations')">← Back</button>

    <h2 style="margin-top:10px">{{ title }}</h2>
    <p class="small">Conversation ID: {{ id }}</p>

    <p v-if="err" class="error">{{ err }}</p>

    <!-- REPLY PANEL -->
    <div v-if="replyTo" class="panel">
      <b>Replying to:</b>
      <code>{{ replyTo }}</code>
      <span class="small"> — {{ replyPreview(replyTo) }}</span>
      <button class="mini" @click="clearReply">Cancel</button>
    </div>

    <!-- FORWARD PANEL -->
    <div v-if="forwarding.messageId" class="panel">
      <b>Forward message:</b> <code>{{ forwarding.messageId }}</code>
      <div class="row">
        <select v-model="forwarding.targetConversationId">
          <option value="">-- choose conversation --</option>
          <option
            v-for="c in myConversations"
            :key="c.id"
            :value="c.id"
          >
            {{ c.title }} {{ c.isGroup ? "(group)" : "(direct)" }}
          </option>
        </select>
        <button
          class="mini"
          @click="confirmForward"
          :disabled="!forwarding.targetConversationId"
        >
          Forward
        </button>
        <button class="mini" @click="cancelForward">Cancel</button>
      </div>
    </div>

    <div v-if="loading">Loading…</div>

    <div v-else class="box">
      <div v-for="m in messages" :key="m.id" class="msg">
        <div class="top">
          <div>
            <b>{{ m.sender }}</b>
            <span class="time">{{ formatTime(m.time) }}</span>
          </div>

          <div class="actions">
            <button class="mini" @click="setReply(m.id)">Reply</button>
            <button class="mini" @click="startForward(m.id)">Forward</button>

            <button
              v-if="isMine(m)"
              class="mini danger"
              @click="deleteMsg(m.id)"
            >
              Delete
            </button>
          </div>
        </div>

        <!-- FORWARD INFO -->
        <div v-if="m.forwardedBy" class="info">
          Forwarded by {{ m.forwardedBy }}
        </div>

        <!-- REPLY INFO -->
        <div v-if="m.replyTo" class="reply">
          Reply to <code>{{ m.replyTo }}</code> — {{ replyPreview(m.replyTo) }}
        </div>

        <!-- MESSAGE BODY -->
        <div v-if="m.type === 'text'">
          {{ m.text }}
        </div>

        <div v-else>
          <img v-if="m.media" :src="m.media" class="img" />
          <i v-else>[image]</i>
        </div>

        <!-- REACTIONS -->
        <div v-if="m.reactions?.length" class="reactions">
          <span v-for="r in m.reactions" :key="r.id">
            {{ r.emoji }} <span class="small">({{ r.user }})</span>
          </span>
        </div>

        <div class="reactions">
          <button class="mini" @click="react(m.id,'👍')">👍</button>
          <button class="mini" @click="react(m.id,'😂')">😂</button>
          <button class="mini" @click="react(m.id,'❤️')">❤️</button>

          <button
            v-if="myReactionId(m)"
            class="mini"
            @click="unreact(m.id, myReactionId(m))"
          >
            Remove my reaction
          </button>
        </div>

        <!-- READ / DELIVERED -->
        <div class="status">
          <span v-if="m.read">✓✓</span>
          <span v-else-if="m.delivered">✓</span>
        </div>
      </div>
    </div>

    <!-- SEND TEXT -->
    <form class="send" @submit.prevent="sendText">
      <input v-model="text" placeholder="Message…" />
      <button>Send</button>
    </form>

    <!-- SEND IMAGE -->
    <div class="send">
      <input type="file" accept="image/*" @change="pickFile" />
      <button @click="sendImage" :disabled="!pickedBase64">Send Image</button>
      <button @click="clearPicked" :disabled="!pickedBase64">Clear</button>
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
      messages: [],
      text: "",
      pickedBase64: "",
      loading: true,
      err: "",
      replyTo: null,
      myConversations: [],
      forwarding: { messageId: "", targetConversationId: "" },
      timer: null,
      myUsername: localStorage.getItem("wasa_username"),
    };
  },

  async mounted() {
    await this.load();
    await this.loadMyConversations();
    this.timer = setInterval(this.load, 1500);
  },

  unmounted() {
    clearInterval(this.timer);
  },

  methods: {
    async load() {
      try {
        const res = await api.get(`/conversations/${this.id}`);
        this.title = res.data.title;
        this.messages = res.data.messages || [];
        this.loading = false;
      } catch {
        this.err = "Failed to load conversation";
        this.loading = false;
      }
    },

    async loadMyConversations() {
      const res = await api.get("/conversations");
      this.myConversations = res.data.conversations || [];
    },

    formatTime(t) {
      return new Date(t).toLocaleTimeString();
    },

    replyPreview(id) {
      const m = this.messages.find(x => x.id === id);
      if (!m) return "unknown";
      return m.type === "image" ? "📷 Photo" : m.text?.slice(0, 40);
    },

    setReply(id) { this.replyTo = id; },
    clearReply() { this.replyTo = null; },

    async sendText() {
      if (!this.text.trim()) return;
      await api.post(`/conversations/${this.id}/messages`, {
        type: "text",
        text: this.text,
        replyTo: this.replyTo,
      });
      this.text = "";
      this.replyTo = null;
      this.load();
    },

    pickFile(e) {
      const f = e.target.files[0];
      const r = new FileReader();
      r.onload = () => this.pickedBase64 = r.result;
      r.readAsDataURL(f);
    },

    clearPicked() { this.pickedBase64 = ""; },

    async sendImage() {
      await api.post(`/conversations/${this.id}/messages`, {
        type: "image",
        media: this.pickedBase64,
        replyTo: this.replyTo,
      });
      this.clearPicked();
      this.replyTo = null;
      this.load();
    },

    isMine(m) { return m.sender === this.myUsername; },

    async deleteMsg(id) {
      await api.delete(`/messages/${id}`);
      this.load();
    },

    startForward(id) {
      this.forwarding.messageId = id;
    },

    cancelForward() {
      this.forwarding.messageId = "";
      this.forwarding.targetConversationId = "";
    },

    async confirmForward() {
      await api.post(`/messages/${this.forwarding.messageId}/forward`, {
        conversationId: this.forwarding.targetConversationId,
      });
      this.cancelForward();
    },

    myReactionId(m) {
      return m.reactions?.find(r => r.user === this.myUsername)?.id;
    },

    async react(id, emoji) {
      await api.post(`/messages/${id}/comments`, { emoji });
      this.load();
    },

    async unreact(id, rid) {
      await api.delete(`/messages/${id}/comments/${rid}`);
      this.load();
    },
  },
};
</script>

<style>
.box { border:1px solid #ddd; padding:10px; max-height:60vh; overflow:auto }
.msg { border-bottom:1px solid #eee; padding:8px }
.top { display:flex; justify-content:space-between }
.actions { display:flex; gap:6px }
.mini { font-size:12px }
.danger { color:red }
.reply, .info { font-size:12px; opacity:0.7 }
.reactions { margin-top:6px }
.status { text-align:right; font-size:12px; opacity:0.6 }
.img { max-width:300px }
.error { color:red }
.panel { background:#fafafa; padding:8px; margin:8px 0 }
.small { font-size:12px; opacity:0.6 }
.send { display:flex; gap:8px; margin-top:8px }
.time { font-size:12px; opacity:0.6 }
</style>
