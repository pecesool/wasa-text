<template>
  <div>
    <h2>Conversations</h2>
    <button type="button" @click="$router.push('/settings')">My Settings</button>
    <!-- Users dropdown -->
    <div class="row">
      <button type="button" @click="loadUsers">Refresh Users</button>

      <select v-model="selectedUser">
        <option value="">-- choose user --</option>
        <option v-for="u in users" :key="u" :value="u">{{ u }}</option>
      </select>

      <button type="button" @click="createDirectFromSelect" :disabled="!selectedUser">
        Start Direct
      </button>
    </div>

    <!-- Manual direct -->
    <div class="row">
      <input v-model="directUser" placeholder="...or type username manually" />
      <button type="button" @click="createDirect">Create Direct</button>
    </div>

    <!-- Create group -->
    <div class="row">
      <input v-model="groupName" placeholder="Group name..." />
      <input v-model="groupMembers" placeholder="Members comma-separated (Luigi,Mario)..." />
      <button type="button" @click="createGroup">Create Group</button>
    </div>

    <p v-if="err" style="color:red">{{ err }}</p>

    <div v-if="loading">Loading...</div>

    <ul v-else class="list">
      <li v-for="c in conversations" :key="c.id" class="item" @click="openChat(c.id)">
        <!-- Avatar -->
        <div class="avatar">
          <img v-if="c.photo" :src="c.photo" alt="avatar" />
          <div v-else class="avatar-fallback"></div>
        </div>

        <!-- Main info -->
        <div class="main">
          <div class="top">
            <div class="title">
              <b>{{ c.title }}</b>
              <span class="kind">{{ c.isGroup ? "group" : "direct" }}</span>
            </div>
            <div class="time">{{ formatTime(c.lastTime) }}</div>
          </div>

          <div class="preview">
            <span v-if="isPhotoPreview(c.lastPreview)">📷 Photo</span>
            <span v-else>{{ c.lastPreview }}</span>
          </div>
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
      users: [],
      selectedUser: "",

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
    await this.loadUsers();
  },
  methods: {
    async load() {
      this.err = "";
      this.loading = true;
      try {
        const res = await api.get("/conversations");
        this.conversations = res.data.conversations || [];
      } catch (e) {
        this.err = "Failed to load conversations";
      } finally {
        this.loading = false;
      }
    },

    async loadUsers() {
      this.err = "";
      try {
        const res = await api.get("/users");
        this.users = res.data.users || [];
      } catch (e) {
        // not fatal
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

    isPhotoPreview(s) {
      return (s || "").trim() === "📷 Photo" || (s || "").trim() === "[photo]";
    },

    async createDirectFromSelect() {
      if (!this.selectedUser) return;
      this.directUser = this.selectedUser;
      await this.createDirect();
    },

    async createDirect() {
      this.err = "";
      const u = this.directUser.trim();
      if (!u) return;

      try {
        const res = await api.post("/conversations", { username: u });
        const id = res.data.conversationId;
        this.directUser = "";
        this.selectedUser = "";
        await this.load();
        this.openChat(id);
      } catch (e) {
        this.err = "Cannot create direct chat";
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
        const id = res.data.conversationId;
        this.groupName = "";
        this.groupMembers = "";
        await this.load();
        this.openChat(id);
      } catch (e) {
        this.err = "Cannot create group";
      }
    },
  },
};
</script>

<style>
.row {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
  align-items: center;
}
input { padding: 6px 10px; }
select { padding: 6px 10px; }

.list { padding: 0; margin: 0; }

.item {
  list-style: none;
  display: flex;
  gap: 12px;
  align-items: center;
  border: 1px solid #ddd;
  padding: 10px;
  border-radius: 8px;
  margin-bottom: 10px;
  cursor: pointer;
}

.avatar {
  width: 46px;
  height: 46px;
  border-radius: 6px;
  overflow: hidden;
  flex: 0 0 auto;
  border: 1px solid #eee;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.avatar-fallback {
  width: 100%;
  height: 100%;
  background: #000;
}

.main { flex: 1; min-width: 0; }

.top {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: baseline;
}

.title {
  display: flex;
  gap: 8px;
  align-items: baseline;
  min-width: 0;
}

.kind {
  font-size: 12px;
  opacity: 0.6;
}

.time {
  font-size: 12px;
  opacity: 0.6;
  white-space: nowrap;
}

.preview {
  margin-top: 4px;
  opacity: 0.8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
