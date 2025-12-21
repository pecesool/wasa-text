import { createRouter, createWebHistory } from "vue-router";
import LoginView from "./views/LoginView.vue";
import ConversationsView from "./views/ConversationsView.vue";
import ChatView from "./views/ChatView.vue";

const routes = [
  { path: "/", redirect: "/login" },
  { path: "/login", component: LoginView },
  { path: "/conversations", component: ConversationsView },
  { path: "/chat/:id", component: ChatView, props: true },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});