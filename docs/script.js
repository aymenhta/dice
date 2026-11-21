
const commands = {
  General: [
    "HELLO", "PING", "ECHO", "EXISTS", "DEL",
    "EXPIRE", "TTL", "TYPE", "DBSIZE", "FLUSHDB",
    "FLUSHALL", "SELECT", "QUIT"
  ],

  Strings: [
    "SET", "GET", "INCR", "DECR", "INCRBY",
    "DECRBY", "MGET", "MSET", "GETDEL", "DELEX",
    "GETEX", "STRLEN", "APPEND", "DIGEST",
    "GETRANGE", "SETRANGE"
  ],

  Hashes: [
    "HSET", "HGET", "HGETALL", "HDEL", "HEXISTS",
    "HLEN", "HMGET", "HKEYS", "HVALS", "HINCRBY",
    "HEXPIRE"
  ],

  Lists: [
    "LPUSH", "RPUSH", "LPUSHX", "RPUSHX", "LPOP",
    "RPOP", "LRANGE", "LLEN", "LINDEX", "LREM",
    "LSET", "LMOVE", "LTRIM"
  ]
};

const container = document.getElementById("commandContainer");
const search = document.getElementById("commandSearch");

function renderCommands() {
  const query = search.value.trim().toUpperCase();

  container.innerHTML = "";

  Object.entries(commands).forEach(([category, items]) => {
    let filtered = items;
    if (query !== "") {
        filtered = items.filter(command => command.includes(query));
    }

    if (!filtered.length) return;

    const column = document.createElement("div");
    column.className = "col-md-6";

    const card = document.createElement("div");
    card.className = "command-card";

    const title = document.createElement("h3");
    title.textContent = `${category} (${filtered.length})`;

    const list = document.createElement("div");
    list.className = "command-list";

    filtered.forEach(command => {
      const item = document.createElement("span");
      item.className = "command-item";
      item.textContent = command;
      list.appendChild(item);
    });

    card.append(title, list);
    column.appendChild(card);
    container.appendChild(column);
  });
}

search.addEventListener("input", renderCommands);

document.getElementById("copyCommand").addEventListener("click", async () => {
  const text = document.getElementById("installCommand").textContent;

  try {
    await navigator.clipboard.writeText(text);
    document.getElementById("copyCommand").textContent = "Copied!";
  } catch {
    document.getElementById("copyCommand").textContent = "Copy failed";
  }
});

renderCommands();



const downloads = {
  windows: {
    label: "Download for Windows",
    url: "https://github.com/yourusername/nexuskv/releases/latest/download/nexuskv-windows-x64.zip"
  },
  macos: {
    label: "Download for macOS",
    url: "https://github.com/yourusername/nexuskv/releases/latest/download/nexuskv-macos-x64.tar.gz"
  },
  linux: {
    label: "Download for Linux",
    url: "https://github.com/yourusername/nexuskv/releases/latest/download/nexuskv-linux-x64.tar.gz"
  }
};

const osSelect = document.getElementById("os-select");
const downloadBtn = document.getElementById("download-btn");

function updateDownload() {
  const selectedOS = osSelect.value;
  const download = downloads[selectedOS];

  downloadBtn.href = download.url;
  downloadBtn.textContent = download.label;
}

osSelect.addEventListener("change", updateDownload);

// Set the initial download option
updateDownload();
