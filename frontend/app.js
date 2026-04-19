const api = {
  auth: "/api/auth",
  user: "/api/user",
  friend: "/api/friend",
};

const authScreen = document.querySelector("#authScreen");
const appScreen = document.querySelector("#appScreen");
const loginForm = document.querySelector("#loginForm");
const registerForm = document.querySelector("#registerForm");
const authNotice = document.querySelector("#authNotice");
const showLoginBtn = document.querySelector("#showLoginBtn");
const showRegisterBtn = document.querySelector("#showRegisterBtn");
const logoutBtn = document.querySelector("#logoutBtn");
const refreshAppBtn = document.querySelector("#refreshAppBtn");
const viewerMini = document.querySelector("#viewerMini");
const statusLine = document.querySelector("#statusLine");
const meCard = document.querySelector("#meCard");
const usersList = document.querySelector("#usersList");
const incomingList = document.querySelector("#incomingList");
const friendsList = document.querySelector("#friendsList");
const recommendationsList = document.querySelector("#recommendationsList");

const ACCESS_TOKEN_KEY = "petbro_access_token";
let currentAuth = null;
let currentUser = null;

function token() {
  return localStorage.getItem(ACCESS_TOKEN_KEY) || "";
}

function setToken(value) {
  if (value) {
    localStorage.setItem(ACCESS_TOKEN_KEY, value);
  } else {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
  }
}

function setStatus(text) {
  statusLine.textContent = text;
}

function setAuthNotice(text, isError = false) {
  authNotice.textContent = text;
  authNotice.style.color = isError ? "#7e2620" : "";
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

async function request(service, path, options = {}) {
  const headers = {
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };

  if (token() && options.auth !== false && !headers.Authorization) {
    headers.Authorization = `Bearer ${token()}`;
  }

  const response = await fetch(api[service] + path, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
    credentials: "include",
  });

  const text = await response.text();
  let data = text;

  try {
    data = text ? JSON.parse(text) : {};
  } catch (_) {
    // keep plain text
  }

  if (!response.ok) {
    throw new Error(typeof data === "string" ? data : `request failed (${response.status})`);
  }

  return data;
}

function showAuth(mode = "login") {
  authScreen.hidden = false;
  appScreen.hidden = true;
  loginForm.hidden = mode !== "login";
  registerForm.hidden = mode !== "register";
  showLoginBtn.classList.toggle("active", mode === "login");
  showRegisterBtn.classList.toggle("active", mode === "register");
}

function showApp() {
  authScreen.hidden = true;
  appScreen.hidden = false;
}

function activateView(viewId) {
  document.querySelectorAll(".nav-btn").forEach((button) => {
    button.classList.toggle("active", button.dataset.view === viewId);
  });

  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("active", view.id === viewId);
  });
}

function renderViewerMini() {
  if (!currentAuth || !currentUser) {
    viewerMini.textContent = "not loaded";
    return;
  }

  viewerMini.innerHTML = `<strong>/${escapeHtml(currentUser.username || "anon")}/</strong><br><span>${escapeHtml(
    currentAuth.email || "",
  )}</span>`;
}

function renderProfile() {
  if (!currentAuth || !currentUser) {
    meCard.className = "muted";
    meCard.textContent = "profile not loaded";
    return;
  }

  meCard.className = "listing";
  meCard.innerHTML = `
    <article class="profile-card">
      <div class="profile-name">/${escapeHtml(currentUser.username || "anon")}/</div>
      <div>${escapeHtml(currentUser.description || "no description yet")}</div>
      <div class="mono">${escapeHtml(currentUser.avatar || "no avatar url")}</div>
      <div class="profile-meta">user #${currentAuth.user_id} / ${escapeHtml(currentAuth.email)}</div>
    </article>
  `;

  document.querySelector("#updateUserForm input[name='id']").value = currentAuth.user_id;
  document.querySelector("#updateUserForm input[name='username']").value = currentUser.username || "";
  document.querySelector("#updateUserForm textarea[name='description']").value = currentUser.description || "";
  document.querySelector("#updateUserForm input[name='avatar']").value = currentUser.avatar || "";
}

function renderUsers(users) {
  if (!Array.isArray(users) || users.length === 0) {
    usersList.className = "listing muted";
    usersList.textContent = "directory is empty or not loaded";
    return;
  }

  usersList.className = "listing";
  usersList.innerHTML = users
    .map((user) => {
      const isMe = Number(user.id) === Number(currentAuth?.user_id);
      return `
        <article class="user-card">
          <div><strong>#${user.id}</strong> /${escapeHtml(user.username || "anon")}/</div>
          <div>${escapeHtml(user.description || "no description")}</div>
          <div class="mono">${escapeHtml(user.avatar || "no avatar url")}</div>
          <div class="toolbar single-line">
            ${isMe ? '<span class="muted">this is you</span>' : `<button type="button" data-send-request="${user.id}">add friend</button>`}
          </div>
        </article>
      `;
    })
    .join("");
}

function renderIncoming(items) {
  if (!Array.isArray(items) || items.length === 0) {
    incomingList.className = "listing muted";
    incomingList.textContent = "no incoming requests";
    return;
  }

  incomingList.className = "listing";
  incomingList.innerHTML = items
    .map(
      (item) => `
        <article class="friend-card">
          <div>from user <strong>#${item.fromUserId}</strong></div>
          <div class="mono">${escapeHtml(item.createdAt || "timestamp missing")}</div>
          <div class="toolbar single-line">
            <button type="button" data-accept-request="${item.fromUserId}">accept</button>
          </div>
        </article>
      `,
    )
    .join("");
}

function renderFriends(items) {
  if (!Array.isArray(items) || items.length === 0) {
    friendsList.className = "listing muted";
    friendsList.textContent = "no friends yet";
    return;
  }

  friendsList.className = "listing";
  friendsList.innerHTML = items
    .map(
      (item) => `
        <article class="friend-card">
          <div>friend id <strong>#${item.id}</strong></div>
        </article>
      `,
    )
    .join("");
}

function renderRecommendations(items) {
  if (!Array.isArray(items) || items.length === 0) {
    recommendationsList.className = "listing muted";
    recommendationsList.textContent = "no recommendations right now";
    return;
  }

  recommendationsList.className = "listing";
  recommendationsList.innerHTML = items
    .map(
      (item) => `
        <article class="friend-card">
          <div>user <strong>#${item.userId}</strong></div>
          <div>mutual friends: ${item.mutualFriends}</div>
          <div class="toolbar single-line">
            <button type="button" data-send-request="${item.userId}">send request</button>
          </div>
        </article>
      `,
    )
    .join("");
}

async function loadMe() {
  currentAuth = await request("auth", "/me");
  currentUser = await request("user", `/user/${currentAuth.user_id}`);
  renderViewerMini();
  renderProfile();
}

async function refreshMainPage() {
  setStatus("loading profile and lists...");
  await loadMe();

  const [incoming, friends, recommendations] = await Promise.all([
    request("friend", "/friend-requests/incoming"),
    request("friend", "/friends"),
    request("friend", "/friend-recommendations?limit=10"),
  ]);

  renderIncoming(incoming);
  renderFriends(friends);
  renderRecommendations(recommendations);
  setStatus("main page refreshed");
}

async function loadUsers(page = "1", limit = "20", sort = "id", order = "asc") {
  const params = new URLSearchParams({ page, limit, sort, order });
  const users = await request("user", `/users?${params.toString()}`);
  renderUsers(users);
}

showLoginBtn.addEventListener("click", () => showAuth("login"));
showRegisterBtn.addEventListener("click", () => showAuth("register"));

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(loginForm);

  try {
    setAuthNotice("logging in...");
    const data = await request("auth", "/login", {
      method: "POST",
      auth: false,
      body: {
        email: form.get("email"),
        password: form.get("password"),
      },
    });

    setToken(data.access_token || "");
    showApp();
    activateView("friendsView");
    await refreshMainPage();
  } catch (error) {
    setAuthNotice(error.message, true);
  }
});

registerForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(registerForm);

  try {
    setAuthNotice("making account...");
    await request("auth", "/register", {
      method: "POST",
      auth: false,
      body: {
        email: form.get("email"),
        password: form.get("password"),
        name: form.get("name"),
        description: form.get("description"),
        avatar: form.get("avatar"),
      },
    });

    setAuthNotice("account created, now log in");
    showAuth("login");
    loginForm.querySelector("input[name='email']").value = String(form.get("email") || "");
    loginForm.querySelector("input[name='password']").value = String(form.get("password") || "");
  } catch (error) {
    setAuthNotice(error.message, true);
  }
});

logoutBtn.addEventListener("click", async () => {
  try {
    await request("auth", "/logout", { method: "POST", auth: false });
  } catch (_) {
    // ignore logout failures, local session still gets cleared
  }

  setToken("");
  currentAuth = null;
  currentUser = null;
  showAuth("login");
  setStatus("logged out");
});

refreshAppBtn.addEventListener("click", async () => {
  try {
    await refreshMainPage();
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelectorAll(".nav-btn").forEach((button) => {
  button.addEventListener("click", () => activateView(button.dataset.view));
});

document.querySelector("#incomingBtn").addEventListener("click", async () => {
  try {
    renderIncoming(await request("friend", "/friend-requests/incoming"));
    setStatus("incoming requests updated");
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#friendsBtn").addEventListener("click", async () => {
  try {
    renderFriends(await request("friend", "/friends"));
    setStatus("friends updated");
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#recommendationsBtn").addEventListener("click", async () => {
  try {
    renderRecommendations(await request("friend", "/friend-recommendations?limit=10"));
    setStatus("recommendations updated");
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#usersQueryForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(event.currentTarget);

  try {
    await loadUsers(form.get("page"), form.get("limit"), form.get("sort"), form.get("order"));
    setStatus("directory updated");
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#userByIdForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(event.currentTarget);

  try {
    const user = await request("user", `/user/${form.get("id")}`);
    renderUsers([user]);
    setStatus(`loaded user #${form.get("id")}`);
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#updateUserForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const payload = {};

  ["username", "description", "avatar"].forEach((key) => {
    const value = String(form.get(key) || "").trim();
    if (value) {
      payload[key] = value;
    }
  });

  try {
    await request("user", `/user/${form.get("id")}`, {
      method: "PUT",
      body: payload,
    });
    await loadMe();
    setStatus("profile saved");
  } catch (error) {
    setStatus(error.message);
  }
});

document.querySelector("#deleteMeBtn").addEventListener("click", async () => {
  const id = document.querySelector("#updateUserForm input[name='id']").value;
  if (!id) {
    return;
  }

  try {
    await request("user", `/user/${id}`, { method: "DELETE" });
    setStatus("profile deleted");
  } catch (error) {
    setStatus(error.message);
  }
});

document.addEventListener("click", async (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return;
  }

  if (target.dataset.sendRequest) {
    try {
      await request("friend", "/friend-request", {
        method: "POST",
        body: {
          toUserId: Number(target.dataset.sendRequest),
        },
      });
      await refreshMainPage();
      setStatus(`friend request sent to #${target.dataset.sendRequest}`);
    } catch (error) {
      setStatus(error.message);
    }
  }

  if (target.dataset.acceptRequest) {
    try {
      await request("friend", "/friend-request/accept", {
        method: "POST",
        body: {
          fromUserId: Number(target.dataset.acceptRequest),
        },
      });
      await refreshMainPage();
      setStatus(`accepted request from #${target.dataset.acceptRequest}`);
    } catch (error) {
      setStatus(error.message);
    }
  }
});

async function bootstrap() {
  showAuth("login");
  setStatus("idle");

  if (!token()) {
    return;
  }

  try {
    showApp();
    activateView("friendsView");
    await refreshMainPage();
  } catch (_) {
    setToken("");
    showAuth("login");
  }
}

bootstrap();
