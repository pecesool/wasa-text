<template>
  <div>
    <h2>Conversations</h2>

    <div class="row">
      <input v-model="directUser" placeholder="Start chat with username..." />
      <button @click="createDirect">Create Direct</button>
    </div>

    <div class="row">
      <input v-model="groupName" placeholder="Group name..." />
      <input v-model="groupMembers" placeholder="Members comma-separated (Luigi,Mario)..." />
      <button @click="createGroup">Create Group</button>
    </div>

    <p v-if="err" style="color:red">{{ err }}</p>

    <div v-if="loading">Loading...</div>

    <ul v-else>
      <li v-for="c in conversations" :key="c.id" class="item">
        <div class="click" @click="openChat(c.id)">
          <b>{{ c.title }}</b>
          <span style="margin-left:8px; opacity:0.7">{{ c.isGroup ? "(group)" : "(direct)" }}</span>
          <div style="opacity:0.8">{{ c.lastPreview }}</div>
          <div style="font-size:12px; opacity:0.6">{{ formatTime(c.lastTime) }}</div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script>
import api, { isLogged } from "../api";

export default {
  data() {
    return {
      conversations: [],
      loading: true,
      err: "",
      directUser: "",
      groupName: "",
      groupMembers: "",
    };
  },
  async mounted() {
    if (!isLogged()) {
      this.$router.push("/login");
      return;
    }
    await this.load();
  },
  methods: {
    async load() {
      this.err = "";
      this.loading = true;
      try {
        const res = await api.get("/conversations");
        this.conversations = res.data.conversations || [];
      } catch (e) {
        this.err = "Failed to load conversations (are you logged?)";
      } finally {
        this.loading = false;
      }
    },
    openChat(id) {
      this.$router.push(`/chat/${id}`);
    },
    formatTime(t) {
      try {
        return new Date(t).toLocaleString();
      } catch {
        return "";
      }
    },
    async createDirect() {
      this.err = "";
      const u = this.directUser.trim();
      if (!u) return;
      try {
        const res = await api.post("/conversations", { username: u });
        this.directUser = "";
        const id = res.data.conversationId;
        await this.load();
        this.openChat(id);
      } catch (e) {
        this.err = "Cannot create direct chat (user missing?)";
      }
    },
    async createGroup() {
      this.err = "";
      const name = this.groupName.trim();
      if (!name) {
        this.err = "Group name required";
        return;
      }
      const members = this.groupMembers
        .split(",")
        .map((x) => x.trim())
        .filter((x) => x.length > 0);

      try {
        const res = await api.post("/groups", { name, members });
        this.groupName = "";
        this.groupMembers = "";
        const id = res.data.conversationId;
        await this.load();
        this.openChat(id);
      } catch (e) {
        this.err = "Cannot create group (check members exist)";
      }
    },
  },
};
</script>

<style>
.row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
.item { list-style: none; margin-bottom: 10px; border: 1px solid #ddd; padding: 10px; border-radius: 6px; }
.click { cursor: pointer; }
</style>
