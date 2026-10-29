"use strict";
var reportCharts = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // src/utils/reportCharts.ts
  var reportCharts_exports = {};
  __export(reportCharts_exports, {
    geneDendrogramOption: () => geneDendrogramOption,
    meanConnectivityOption: () => meanConnectivityOption,
    moduleTraitHeatmapOption: () => moduleTraitHeatmapOption,
    sampleTreeOption: () => sampleTreeOption,
    scaleIndependenceOption: () => scaleIndependenceOption,
    sftHistogramOption: () => sftHistogramOption,
    sftLogLogOption: () => sftLogLogOption
  });

  // src/utils/dendrogramLayout.ts
  function layoutDendrogram(merge, height, order, hang = 0.1) {
    const n = merge.length + 1;
    const leafX = new Array(n).fill(0);
    order.forEach((leaf, pos) => {
      leafX[leaf] = pos;
    });
    const nodeX = new Array(2 * n - 1).fill(0);
    const nodeY = new Array(2 * n - 1).fill(0);
    for (let i = 0; i < n; i++) {
      nodeX[i] = leafX[i];
      nodeY[i] = 0;
    }
    const minH = height.length ? Math.min(...height) : 0;
    const maxH = height.length ? Math.max(...height) : 1;
    const hangScaled = hang * (maxH - minH);
    const hSegments = [];
    const vSegments = [];
    const childIdx = (x) => x < 0 ? -x - 1 : n + x - 1;
    for (let i = 0; i < merge.length; i++) {
      const a = merge[i][0];
      const b = merge[i][1];
      const ca = childIdx(a);
      const cb = childIdx(b);
      const xa = nodeX[ca];
      const xb = nodeX[cb];
      const h = height[i];
      const ya = a < 0 ? h - hangScaled : nodeY[ca];
      const yb = b < 0 ? h - hangScaled : nodeY[cb];
      const k = n + i;
      nodeX[k] = (xa + xb) / 2;
      nodeY[k] = h;
      hSegments.push([xa, xb, h]);
      vSegments.push([xa, ya, h], [xb, yb, h]);
    }
    return { n, leafX, nodeX, nodeY, hSegments, vSegments, minH, maxH, hangScaled };
  }

  // src/utils/colors.ts
  var MODULE_COLORS = {
    turquoise: "#1f9e89",
    blue: "#3182bd",
    brown: "#8c510a",
    yellow: "#e6ab02",
    green: "#66a61e",
    red: "#e41a1c",
    black: "#252525",
    pink: "#f781bf",
    magenta: "#984ea3",
    purple: "#6a51a3",
    greenyellow: "#a6d854",
    tan: "#b89b74",
    salmon: "#fa8072",
    cyan: "#00ced1",
    midnightblue: "#191970",
    lightcyan: "#e0ffff",
    grey: "#999999",
    grey60: "#999999",
    lightgreen: "#90ee90",
    lightyellow: "#ffffe0",
    royalblue: "#4169e1",
    darkred: "#8b0000",
    darkgreen: "#006400",
    darkturquoise: "#00ced1",
    darkgrey: "#a9a9a9",
    orange: "#ffa500",
    darkorange: "#ff8c00",
    white: "#ffffff",
    skyblue: "#87ceeb",
    saddlebrown: "#8b4513",
    steelblue: "#4682b4",
    paleturquoise: "#afeeee",
    violet: "#ee82ee",
    darkolivegreen: "#556b2f",
    darkmagenta: "#8b008b",
    sienna3: "#cd5c5c",
    yellowgreen: "#9acd32",
    skyblue3: "#6ca6cd",
    plum1: "#ffbbff",
    orangered4: "#801700",
    mediumpurple3: "#8968cd",
    lightsteelblue1: "#cae1ff",
    lightcyan1: "#e0ffff",
    ivory: "#fffff0",
    floralwhite: "#fffaf0",
    darkorange2: "#ee7600",
    brown4: "#8b2323",
    bisque4: "#8b7d6b",
    darkslateblue: "#483d8b",
    plum2: "#eeaeee",
    thistle2: "#eed2ee"
  };
  function moduleColor(name) {
    var _a;
    return (_a = MODULE_COLORS[name]) != null ? _a : "#cccccc";
  }

  // src/utils/reportCharts.ts
  var PALETTE = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b", "#e377c2", "#7f7f7f", "#bcbd22", "#17becf", "#aec7e8", "#ffbb78"];
  function scaleIndependenceOption(fit, rsqCut, title, color = "#5470c6", saveName = "scale_independence", downloadTitle = "Download PNG") {
    return {
      title: { text: title, left: "center", textStyle: { fontSize: 14 } },
      tooltip: { trigger: "axis" },
      toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: "#ffffff" } } },
      xAxis: { type: "category", name: "Soft Threshold (power)", nameLocation: "middle", nameGap: 32, data: fit.map((f) => f.power) },
      yAxis: { type: "value", name: "Scale Free Topology Model Fit, signed R\xB2", nameLocation: "middle", nameRotate: 90, nameGap: 55 },
      series: [{
        type: "line",
        data: fit.map((f) => +f.rsq.toFixed(3)),
        symbolSize: 8,
        lineStyle: { width: 2, color },
        markLine: {
          data: [{ yAxis: rsqCut, label: { formatter: "R\xB2 = " + rsqCut } }],
          lineStyle: { color: "#e11d48", type: "dashed" }
        }
      }]
    };
  }
  function meanConnectivityOption(fit, title, color = "#5470c6", saveName = "mean_connectivity", downloadTitle = "Download PNG") {
    return {
      title: { text: title, left: "center", textStyle: { fontSize: 14 } },
      tooltip: { trigger: "axis" },
      toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: "#ffffff" } } },
      xAxis: { type: "category", name: "Soft Threshold (power)", nameLocation: "middle", nameGap: 32, data: fit.map((f) => f.power) },
      yAxis: { type: "value", name: "Mean Connectivity", nameLocation: "middle", nameRotate: 90, nameGap: 55 },
      series: [{
        type: "line",
        data: fit.map((f) => +f.mean_k.toFixed(3)),
        symbolSize: 8,
        lineStyle: { width: 2, color },
        areaStyle: { opacity: 0.08 }
      }]
    };
  }
  function sftHistogramOption(sc, title, freqName = "Frequency") {
    return {
      title: { text: `${title} (power = ${sc.power})`, left: "center", textStyle: { fontSize: 14 } },
      tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
      grid: { left: 60, right: 20, top: 50, bottom: 40 },
      xAxis: { type: "category", name: "k", nameLocation: "middle", nameGap: 32, data: sc.hist_mids.map((m) => m.toFixed(1)) },
      yAxis: { type: "value", name: freqName, nameLocation: "middle", nameRotate: 90, nameGap: 55 },
      series: [{
        type: "bar",
        data: sc.hist_counts,
        itemStyle: { color: "#94a3b8", borderColor: "#334155", borderWidth: 0.5 },
        barCategoryGap: "5%"
      }]
    };
  }
  function sftLogLogOption(sc, title) {
    const { log_k: x, log_p: y, intercept, slope, adj_r2 } = sc;
    const x0 = Math.min(...x);
    const x1 = Math.max(...x);
    const line = [[x0, intercept + slope * x0], [x1, intercept + slope * x1]];
    return {
      title: {
        text: `${title}
scale R\xB2=${adj_r2.toFixed(2)}, slope=${slope.toFixed(2)}`,
        left: "center",
        textStyle: { fontSize: 13 }
      },
      tooltip: { trigger: "item" },
      grid: { left: 60, right: 20, top: 60, bottom: 40 },
      xAxis: { type: "value", name: "log10(k)", nameLocation: "middle", nameGap: 32 },
      yAxis: { type: "value", name: "log10(p(k))", nameLocation: "middle", nameRotate: 90, nameGap: 55 },
      series: [
        {
          type: "scatter",
          data: x.map((v, i) => [v, y[i]]),
          symbolSize: 9,
          itemStyle: { color: "#5470c6", borderColor: "#1e40af", borderWidth: 1 }
        },
        {
          type: "line",
          data: line,
          symbol: "none",
          lineStyle: { color: "#e11d48", width: 2 }
        }
      ]
    };
  }
  function sampleTreeOption(samples, tree, bands, selected = [], saveName = "sample_cluster", downloadTitle = "Download PNG") {
    const order = tree.order.map((i) => i - 1);
    const sampleNames = order.map((i) => {
      var _a;
      return (_a = samples[i]) != null ? _a : "";
    });
    const selectedSet = new Set(selected);
    const layout = layoutDendrogram(tree.merge, tree.height, order, 0.03);
    const n = layout.n;
    const minH = layout.minH;
    const maxH = layout.maxH;
    const span = Math.max(maxH - minH, 1e-6);
    const lowestLeafY = minH - layout.hangScaled;
    const leafTipY = new Array(n).fill(lowestLeafY);
    for (let i = 0; i < tree.merge.length; i++) {
      const tip = tree.height[i] - layout.hangScaled;
      const a = tree.merge[i][0];
      const b = tree.merge[i][1];
      if (a < 0) leafTipY[-a - 1] = tip;
      if (b < 0) leafTipY[-b - 1] = tip;
    }
    const renderItem = (params, api) => {
      const children = [];
      for (const [x1, x2, y] of layout.hSegments) {
        const a = api.coord([x1, y]);
        const b = api.coord([x2, y]);
        children.push({ type: "line", shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      }
      for (const [x, y1, y2] of layout.vSegments) {
        const a = api.coord([x, y1]);
        const b = api.coord([x, y2]);
        children.push({ type: "line", shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      }
      return { type: "group", children };
    };
    const cats = bands.map((c) => {
      const vals = Object.values(c.values);
      const uniq = [...new Set(vals)].sort();
      const m = {};
      uniq.forEach((v, i) => {
        m[v] = PALETTE[i % PALETTE.length];
      });
      return { name: c.name, values: c.values, palette: m };
    });
    const K = cats.length;
    const grids = [{ left: 110, right: 16, top: "4%", bottom: K > 0 ? 48 + K * 30 : 44 }];
    const xAxes = [{ type: "value", min: -0.5, max: n - 0.5, show: false, gridIndex: 0 }];
    const yAxes = [{
      type: "value",
      min: lowestLeafY - 0.08 * span,
      max: maxH * 1.03,
      name: "Height",
      nameLocation: "middle",
      nameRotate: 90,
      nameGap: 55,
      gridIndex: 0,
      axisLabel: { formatter: (v) => v < minH ? "" : v.toFixed(2) },
      splitLine: { show: true, lineStyle: { color: "#eef1f4", type: "dashed" } }
    }];
    const series = [
      { type: "custom", renderItem, data: [0], xAxisIndex: 0, yAxisIndex: 0, z: 1 },
      {
        type: "scatter",
        xAxisIndex: 0,
        yAxisIndex: 0,
        z: 10,
        data: Array.from({ length: n }, (_, p) => ({ value: [p, leafTipY[order[p]]], name: sampleNames[p] })),
        symbolSize: 9,
        itemStyle: {
          color: (p) => selectedSet.has(p.name) ? "#dc2626" : "#0d9488",
          borderColor: "#ffffff",
          borderWidth: 1.5
        },
        emphasis: { scale: 1.6 }
      }
    ];
    for (let i = 0; i < K; i++) {
      const gi = i + 1;
      const band = cats[i];
      grids.push({ left: 110, right: 16, bottom: 10 + (K - 1 - i) * 30, height: 26 });
      xAxes.push({ type: "value", min: -0.5, max: n - 0.5, show: false, gridIndex: gi });
      yAxes.push({
        type: "value",
        min: 0,
        max: 1,
        gridIndex: gi,
        name: band.name,
        nameLocation: "middle",
        nameGap: 12,
        nameRotate: 0,
        nameTextStyle: { fontSize: 11, color: "#4b5563", align: "right" },
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { show: false },
        splitLine: { show: false }
      });
      series.push({
        type: "custom",
        data: [0],
        xAxisIndex: gi,
        yAxisIndex: gi,
        z: 5,
        renderItem: (params, api) => {
          var _a;
          const children = [];
          for (let p = 0; p < n; p++) {
            const name = sampleNames[p];
            const color = (_a = band.palette[band.values[name]]) != null ? _a : "#d1d5db";
            const x0 = api.coord([p - 0.5, 0])[0];
            const x1 = api.coord([p + 0.5, 0])[0];
            const yTop = api.coord([p, 1])[1];
            const yBot = api.coord([p, 0])[1];
            children.push({ type: "rect", shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } });
          }
          return { type: "group", children };
        }
      });
    }
    return {
      grid: grids,
      xAxis: xAxes,
      yAxis: yAxes,
      tooltip: {
        trigger: "item",
        confine: true,
        backgroundColor: "#0f172a",
        borderWidth: 0,
        textStyle: { color: "#fff", fontSize: 12 },
        formatter: (params) => {
          if (params.seriesType === "scatter") {
            const name = params.name;
            const parts = cats.map((c) => {
              var _a;
              return `${c.name}: ${(_a = c.values[name]) != null ? _a : ""}`;
            });
            return name + (parts.length ? "<br/>" + parts.join("<br/>") : "");
          }
          return "";
        }
      },
      toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: "#ffffff" } } },
      series
    };
  }
  function geneDendrogramOption(genes, colors, dendrogram, downloadTitle = "Download PNG", saveName = "module_dendrogram") {
    const { merge, height } = dendrogram;
    const order = dendrogram.order.map((i) => i - 1);
    const layout = layoutDendrogram(merge, height, order, 0.03);
    const n = layout.n;
    const minH = layout.minH;
    const maxH = layout.maxH;
    const lowestLeafY = minH - layout.hangScaled;
    const barH = (maxH - minH) * 0.3;
    const renderItem = (params, api) => {
      const children = [];
      for (let p = 0; p < n; p++) {
        const geneIdx = order[p];
        const color = moduleColor(colors[geneIdx]);
        const tl = api.coord([p - 0.5, lowestLeafY]);
        const br = api.coord([p + 0.5, lowestLeafY - barH]);
        children.push({
          type: "rect",
          shape: { x: tl[0], y: tl[1], width: br[0] - tl[0], height: br[1] - tl[1] },
          style: { fill: color }
        });
      }
      const bTop1 = api.coord([-0.5, lowestLeafY]);
      const bTop2 = api.coord([n - 0.5, lowestLeafY]);
      children.push({ type: "line", shape: { x1: bTop1[0], y1: bTop1[1], x2: bTop2[0], y2: bTop2[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      const bBot1 = api.coord([-0.5, lowestLeafY - barH]);
      const bBot2 = api.coord([n - 0.5, lowestLeafY - barH]);
      children.push({ type: "line", shape: { x1: bBot1[0], y1: bBot1[1], x2: bBot2[0], y2: bBot2[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      for (const [x1, x2, y] of layout.hSegments) {
        const a = api.coord([x1, y]);
        const b = api.coord([x2, y]);
        children.push({ type: "line", shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      }
      for (const [x, y1, y2] of layout.vSegments) {
        const a = api.coord([x, y1]);
        const b = api.coord([x, y2]);
        children.push({ type: "line", shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: "#111827", lineWidth: 1 } });
      }
      const lp = api.coord([0, lowestLeafY - barH / 2]);
      children.push({ type: "text", style: { text: "Module color", x: 10, y: lp[1], fill: "#6b7280", fontSize: 11, textAlign: "left", textVerticalAlign: "middle" } });
      return { type: "group", children };
    };
    return {
      grid: { left: 80, right: 16, top: 20, bottom: 20 },
      xAxis: { type: "value", min: -0.5, max: n - 0.5, show: false },
      yAxis: {
        type: "value",
        min: lowestLeafY - barH,
        max: maxH * 1.03,
        name: "Height",
        nameLocation: "middle",
        nameRotate: 90,
        nameGap: 55,
        axisLabel: { formatter: (v) => v < minH ? "" : v.toFixed(2) },
        splitLine: { show: true, lineStyle: { color: "#eef1f4", type: "dashed" } }
      },
      toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: "#ffffff" } } },
      series: [{ type: "custom", renderItem, data: [0] }]
    };
  }
  function sigMark(p) {
    if (p < 1e-3) return "***";
    if (p < 0.01) return "**";
    if (p < 0.05) return "*";
    return "";
  }
  function moduleTraitHeatmapOption(modules, traits, cor, p, lowColor = "#2166ac", midColor = "#ffffff", highColor = "#b2182b", downloadTitle = "Download PNG", saveName = "module_trait") {
    const heat = [];
    cor.forEach((row, i) => row.forEach((v, j) => heat.push([j, i, +v.toFixed(4)])));
    return {
      tooltip: {
        formatter: (params) => {
          var _a, _b;
          if (params.seriesType === "bar") return "";
          const xj = params.value[0];
          const yi = params.value[1];
          const v = params.value[2];
          return modules[yi] + " \xD7 " + traits[xj] + "<br/>cor = " + v + "<br/>p = " + ((_b = (_a = p[yi]) == null ? void 0 : _a[xj]) != null ? _b : 0);
        }
      },
      grid: [
        { left: 125, width: 22, bottom: 100, top: 20 },
        { left: 147, right: 30, bottom: 100, top: 20 }
      ],
      xAxis: [
        { type: "value", min: 0, max: 1, show: false, gridIndex: 0 },
        { type: "category", data: traits, gridIndex: 1, axisLabel: { rotate: 45 }, axisLine: { show: false }, axisTick: { show: false } }
      ],
      yAxis: [
        { type: "category", data: modules, gridIndex: 0, axisLine: { show: false }, axisTick: { show: false } },
        { type: "category", data: modules, gridIndex: 1, show: false }
      ],
      visualMap: {
        min: -1,
        max: 1,
        calculable: true,
        orient: "horizontal",
        left: "center",
        bottom: 0,
        inRange: { color: [lowColor, midColor, highColor] },
        seriesIndex: 1
      },
      toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: "#ffffff" } } },
      series: [
        {
          type: "bar",
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: modules.map((m) => ({ value: 1, itemStyle: { color: moduleColor(m), borderColor: "#333", borderWidth: 0.8 } })),
          barWidth: "100%",
          barCategoryGap: "0%",
          silent: true,
          z: 2
        },
        {
          type: "heatmap",
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: heat,
          label: {
            show: true,
            formatter: (params) => {
              var _a, _b, _c, _d;
              const xj = params.value[0];
              const yi = params.value[1];
              const c = (_b = (_a = cor[yi]) == null ? void 0 : _a[xj]) != null ? _b : 0;
              const pv = (_d = (_c = p[yi]) == null ? void 0 : _c[xj]) != null ? _d : 1;
              return c.toFixed(2) + String.fromCharCode(10) + sigMark(pv);
            },
            fontSize: 11,
            lineHeight: 14
          },
          itemStyle: { borderColor: "#333", borderWidth: 0.6 },
          emphasis: { itemStyle: { shadowBlur: 10, shadowColor: "rgba(0,0,0,0.4)" } }
        }
      ]
    };
  }
  return __toCommonJS(reportCharts_exports);
})();
