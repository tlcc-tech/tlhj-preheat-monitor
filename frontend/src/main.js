import "./style.css";
import "./app.css";

import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
  Filler,
} from "chart.js";

import { BrowserOpenURL, EventsOn } from "../wailsjs/runtime/runtime";
import {
  GetAppInfo,
  GetHistory,
  GetSettings,
  GetStatus,
  StartMonitoring,
  StopMonitoring,
} from "../wailsjs/go/main/App";

Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
  Filler,
);

document.querySelector("#app").innerHTML = `
    <div class="container">
        <h1 class="title">新服预约监控</h1>

        <div class="input-box">
            <input class="input" id="channelKey" type="text" autocomplete="off" placeholder="微信单点推送链接（例如：https://xizhi.qqoq.net/XZxxxx.send，可选）" />
            <button class="btn" id="startBtn">开始监控</button>
            <button class="btn" id="stopBtn">结束监控</button>
        </div>

        <div class="result" id="status">状态：加载中...</div>

        <textarea class="log" id="log" readonly spellcheck="false"></textarea>

        <div class="chart-section">
            <div class="charts-row">
                <div class="chart-panel">
                    <p class="chart-title">预约人数走势</p>
                    <div class="chart-wrap">
                        <canvas id="countChart"></canvas>
                    </div>
                </div>
                <div class="chart-panel">
                    <p class="chart-title">每 5 分钟新增人数</p>
                    <div class="chart-wrap">
                        <canvas id="deltaChart"></canvas>
                    </div>
                </div>
            </div>
        </div>

        <div class="footer">
            <div class="footer-left">
                <div>说明：每 5 分钟采集预约人数并保存到本地；每满 1000 人（13000、14000…）微信推送。</div>
                <div>页面：<a href="#" id="pageLink" style="color:#6eb5ff">新服预约活动页</a></div>
                <button class="footer-btn" id="getPushLinkBtn" type="button">如何获取微信推送链接？</button>
                <div>作者：<span id="author"></span>　版本：<span id="version"></span></div>
            </div>
            <div class="footer-right">
                <div class="footer-qrcode-item">
                    <span class="footer-qrcode-label">公众号</span>
                    <img class="footer-img" src="/qrcode_gzh.jpg" alt="公众号二维码" />
                </div>
                <div class="footer-qrcode-item">
                    <span class="footer-qrcode-label">小程序</span>
                    <img class="footer-img" src="/qrcode.jpg" alt="小程序二维码" />
                </div>
            </div>
        </div>
    </div>
`;

const channelKeyEl = document.getElementById("channelKey");
const startBtn = document.getElementById("startBtn");
const stopBtn = document.getElementById("stopBtn");
const statusEl = document.getElementById("status");
const logEl = document.getElementById("log");
const authorEl = document.getElementById("author");
const versionEl = document.getElementById("version");
const getPushLinkBtn = document.getElementById("getPushLinkBtn");
const pageLink = document.getElementById("pageLink");

const ACTIVITY_URL =
  "https://tlhj-activity.changyou.com/tlhj/preheat/20211206/pc/index.shtml";

const MAX_LOG_LINES = 2000;
const logLines = [];

let countChart = null;
let deltaChart = null;
const countLabels = [];
const countData = [];
const deltaLabels = [];
const deltaData = [];

const chartBaseOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { labels: { color: "#e8eef5" } },
  },
  scales: {
    x: {
      ticks: { color: "#b8c5d6", maxTicksLimit: 6 },
      grid: { color: "rgba(255,255,255,0.08)" },
    },
    y: {
      ticks: { color: "#b8c5d6" },
      grid: { color: "rgba(255,255,255,0.08)" },
    },
  },
};

function deltaFromCounts(counts) {
  return counts.map((c, i) => (i === 0 ? 0 : c - counts[i - 1]));
}

function formatRelativeTime(date) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return "";
  const diffMs = Date.now() - date.getTime();
  if (diffMs < 0) return "刚刚";
  const sec = Math.floor(diffMs / 1000);
  if (sec < 10) return "刚刚";
  if (sec < 60) return `${sec}秒前`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}分钟前`;
  const hour = Math.floor(min / 60);
  if (hour < 24) return `${hour}小时前`;
  const day = Math.floor(hour / 24);
  return `${day}天前`;
}

function formatLocalTime(date) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function formatChartLabel(at) {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return at;
  return d.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function appendLog(line) {
  if (!line) return;
  logLines.push(line);
  if (logLines.length > MAX_LOG_LINES) {
    logLines.splice(0, logLines.length - MAX_LOG_LINES);
  }
  logEl.value = logLines.join("\n");
  logEl.scrollTop = logEl.scrollHeight;
}

function setButtons(running) {
  startBtn.disabled = !!running;
  stopBtn.disabled = !running;
}

function initCharts() {
  countChart = new Chart(document.getElementById("countChart"), {
    type: "line",
    data: {
      labels: countLabels,
      datasets: [
        {
          label: "预约人数",
          data: countData,
          borderColor: "rgba(110, 181, 255, 1)",
          backgroundColor: "rgba(110, 181, 255, 0.15)",
          fill: true,
          tension: 0.2,
          pointRadius: 2,
          pointHoverRadius: 5,
        },
      ],
    },
    options: {
      ...chartBaseOptions,
      plugins: {
        ...chartBaseOptions.plugins,
        tooltip: {
          callbacks: {
            label: (ctx) => ` ${ctx.parsed.y} 人`,
          },
        },
      },
      scales: {
        ...chartBaseOptions.scales,
        y: { ...chartBaseOptions.scales.y, beginAtZero: false },
      },
    },
  });

  deltaChart = new Chart(document.getElementById("deltaChart"), {
    type: "line",
    data: {
      labels: deltaLabels,
      datasets: [
        {
          label: "新增人数",
          data: deltaData,
          borderColor: "rgba(255, 167, 94, 1)",
          backgroundColor: "rgba(255, 167, 94, 0.15)",
          fill: true,
          tension: 0.2,
          pointRadius: 2,
          pointHoverRadius: 5,
        },
      ],
    },
    options: {
      ...chartBaseOptions,
      plugins: {
        ...chartBaseOptions.plugins,
        tooltip: {
          callbacks: {
            label: (ctx) => {
              const v = ctx.parsed.y;
              return v >= 0 ? ` +${v} 人` : ` ${v} 人`;
            },
          },
        },
      },
      scales: {
        ...chartBaseOptions.scales,
        y: { ...chartBaseOptions.scales.y, beginAtZero: true },
      },
    },
  });
}

function syncDeltaSeries() {
  const deltas = deltaFromCounts(countData);
  deltaLabels.length = 0;
  deltaData.length = 0;
  for (let i = 0; i < countLabels.length; i++) {
    deltaLabels.push(countLabels[i]);
    deltaData.push(deltas[i]);
  }
}

function refreshCharts() {
  syncDeltaSeries();
  countChart?.update();
  deltaChart?.update();
}

function addChartPoint(at, count) {
  countLabels.push(formatChartLabel(at));
  countData.push(count);
  refreshCharts();
}

function loadHistoryToChart(points) {
  countLabels.length = 0;
  countData.length = 0;
  const sorted = [...(points || [])].filter(
    (p) => p?.at != null && p?.count != null,
  );
  sorted.sort((a, b) => new Date(a.at).getTime() - new Date(b.at).getTime());
  for (const p of sorted) {
    countLabels.push(formatChartLabel(p.at));
    countData.push(p.count);
  }
  refreshCharts();
}

async function refreshStatus() {
  try {
    const s = await GetStatus();
    setButtons(s.running);

    const lines = [`状态：${s.running ? "运行中" : "已停止"}`];

    if (s.lastCount > 0) {
      lines.push(`当前预约人数：${s.lastCount}`);
    }

    if (s.lastChecked) {
      const d = new Date(s.lastChecked);
      const local = formatLocalTime(d);
      const rel = formatRelativeTime(d);
      const extra = rel ? `（${rel}）` : "";
      lines.push(
        local ? `最近采集：${local}${extra}` : `最近采集：${s.lastChecked}`,
      );
    }

    if (s.running && s.nextPollIn != null) {
      const min = Math.floor(s.nextPollIn / 60);
      const sec = s.nextPollIn % 60;
      if (min > 0) {
        lines.push(`下次采集：约 ${min} 分 ${sec} 秒后`);
      } else {
        lines.push(`下次采集：约 ${sec} 秒后`);
      }
    }

    if (s.lastError) {
      lines.push(`最近错误：${s.lastError}`);
    }

    statusEl.textContent = lines.join("\n");
  } catch (e) {
    statusEl.textContent = "状态：获取失败";
    appendLog(String(e));
  }
}

startBtn.addEventListener("click", async () => {
  const key = (channelKeyEl.value || "").trim();
  try {
    await StartMonitoring(key);
    await refreshStatus();
  } catch (e) {
    appendLog(String(e));
  }
});

stopBtn.addEventListener("click", async () => {
  try {
    StopMonitoring();
    await refreshStatus();
  } catch (e) {
    appendLog(String(e));
  }
});

getPushLinkBtn?.addEventListener("click", () => {
  try {
    BrowserOpenURL("https://xz.qqoq.net/");
  } catch (e) {
    appendLog(String(e));
  }
});

pageLink?.addEventListener("click", (e) => {
  e.preventDefault();
  try {
    BrowserOpenURL(ACTIVITY_URL);
  } catch (err) {
    appendLog(String(err));
  }
});

EventsOn("log", (line) => {
  appendLog(line);
});

EventsOn("count", (payload) => {
  const count = payload?.count ?? payload?.Count;
  const at = payload?.at ?? payload?.At;
  if (count != null && at) {
    addChartPoint(at, count);
    refreshStatus();
  }
});

setInterval(() => {
  refreshStatus();
}, 5000);

channelKeyEl.focus();

initCharts();

GetSettings()
  .then((s) => {
    const saved = (s?.channelKey || "").trim();
    if (saved && !(channelKeyEl.value || "").trim()) {
      channelKeyEl.value = saved;
    }
  })
  .catch((e) => appendLog(String(e)));

GetHistory(5000)
  .then((points) => loadHistoryToChart(points))
  .catch((e) => appendLog(String(e)));

GetAppInfo()
  .then((info) => {
    if (authorEl) authorEl.innerText = info.author || "";
    if (versionEl) versionEl.innerText = info.version || "";
  })
  .catch((e) => appendLog(String(e)));

refreshStatus();
