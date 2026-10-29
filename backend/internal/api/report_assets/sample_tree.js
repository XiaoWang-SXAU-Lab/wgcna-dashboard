// Interactive sample tree for the rsWGCNA report (plain JS, no framework).
// Reads {samples, tree:{merge,height,order}, bands:[{name,values}]} from
// #sample-tree-data, renders a dendrogram + one colour band per sample_info key,
// and lets the user toggle which keys are shown.
(function () {
  function initSampleTree() {
    var dataEl = document.getElementById('sample-tree-data');
    if (!dataEl || !window.echarts) return;
    var data = JSON.parse(dataEl.textContent);
    var samples = data.samples || [];
    var tree = data.tree || { merge: [], height: [], order: [] };
    var bands = data.bands || [];

    var PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78'];

    function layoutDendrogram(merge, height, order, hang) {
      var n = merge.length + 1;
      var leafX = new Array(n).fill(0);
      order.forEach(function (leaf, pos) { leafX[leaf] = pos; });
      var nodeX = new Array(2 * n - 1).fill(0);
      var nodeY = new Array(2 * n - 1).fill(0);
      for (var i = 0; i < n; i++) { nodeX[i] = leafX[i]; nodeY[i] = 0; }
      var minH = height.length ? Math.min.apply(null, height) : 0;
      var maxH = height.length ? Math.max.apply(null, height) : 1;
      var hangScaled = hang * (maxH - minH);
      var hSegments = [], vSegments = [];
      function childIdx(x) { return x < 0 ? -x - 1 : n + x - 1; }
      for (var j = 0; j < merge.length; j++) {
        var a = merge[j][0], b = merge[j][1];
        var ca = childIdx(a), cb = childIdx(b);
        var xa = nodeX[ca], xb = nodeX[cb];
        var h = height[j];
        var ya = a < 0 ? h - hangScaled : nodeY[ca];
        var yb = b < 0 ? h - hangScaled : nodeY[cb];
        var k = n + j;
        nodeX[k] = (xa + xb) / 2;
        nodeY[k] = h;
        hSegments.push([xa, xb, h]);
        vSegments.push([xa, ya, h], [xb, yb, h]);
      }
      return { n: n, minH: minH, maxH: maxH, hangScaled: hangScaled, hSegments: hSegments, vSegments: vSegments };
    }

    var order = tree.order.map(function (i) { return i - 1; });
    var sampleNames = order.map(function (i) { return samples[i] || ''; });
    var layout = layoutDendrogram(tree.merge, tree.height, order, 0.03);
    var n = layout.n;

    // leaf tip y (R plot.hclust convention)
    var leafTipY = new Array(n).fill(layout.minH - layout.hangScaled);
    for (var i = 0; i < tree.merge.length; i++) {
      var tip = tree.height[i] - layout.hangScaled;
      var ma = tree.merge[i][0], mb = tree.merge[i][1];
      if (ma < 0) leafTipY[-ma - 1] = tip;
      if (mb < 0) leafTipY[-mb - 1] = tip;
    }

    function palettesFor(keys) {
      return keys.map(function (c) {
        var vals = Object.values(c.values || {});
        var uniq = Array.from(new Set(vals)).sort();
        var m = {};
        uniq.forEach(function (v, idx) { m[v] = PALETTE[idx % PALETTE.length]; });
        return { name: c.name, values: c.values || {}, palette: m };
      });
    }

    function buildOption(active) {
      var cats = active;
      var K = cats.length;
      var grids = [{ left: 110, right: 16, top: '4%', bottom: K > 0 ? 48 + K * 30 : 44 }];
      var xAxes = [{ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: 0 }];
      var yAxes = [{
        type: 'value', min: layout.minH - layout.hangScaled - 0.08 * (layout.maxH - layout.minH), max: layout.maxH * 1.03,
        name: 'Height', nameLocation: 'middle', nameRotate: 90, nameGap: 55, gridIndex: 0,
        axisLabel: { formatter: function (v) { return v < layout.minH ? '' : v.toFixed(2); } },
        splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
      }];
      var series = [{
        type: 'custom', data: [0], xAxisIndex: 0, yAxisIndex: 0, z: 1,
        renderItem: function (params, api) {
          var children = [];
          layout.hSegments.forEach(function (s) {
            var a = api.coord([s[0], s[2]]), b = api.coord([s[1], s[2]]);
            children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } });
          });
          layout.vSegments.forEach(function (s) {
            var a = api.coord([s[0], s[1]]), b = api.coord([s[0], s[2]]);
            children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } });
          });
          return { type: 'group', children: children };
        },
      }, {
        type: 'scatter', xAxisIndex: 0, yAxisIndex: 0, z: 10,
        data: Array.from({ length: n }, function (_, p) { return { value: [p, leafTipY[order[p]]], name: sampleNames[p] }; }),
        symbolSize: 9,
        itemStyle: { color: '#0d9488', borderColor: '#ffffff', borderWidth: 1.5 },
      }];

      for (var i = 0; i < K; i++) {
        var gi = i + 1;
        var band = cats[i];
        grids.push({ left: 110, right: 16, bottom: 10 + (K - 1 - i) * 30, height: 26 });
        xAxes.push({ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: gi });
        yAxes.push({
          type: 'value', min: 0, max: 1, gridIndex: gi,
          name: band.name, nameLocation: 'middle', nameGap: 12,
          nameRotate: 0,
          nameTextStyle: { fontSize: 11, color: '#4b5563', align: 'right' },
          axisLine: { show: false }, axisTick: { show: false }, axisLabel: { show: false }, splitLine: { show: false },
        });
        series.push((function (band) {
          return {
            type: 'custom', data: [0], xAxisIndex: gi, yAxisIndex: gi, z: 5,
            renderItem: function (params, api) {
              var children = [];
              for (var p = 0; p < n; p++) {
                var name = sampleNames[p];
                var color = band.palette[band.values[name]] || '#d1d5db';
                var x0 = api.coord([p - 0.5, 0])[0];
                var x1 = api.coord([p + 0.5, 0])[0];
                var yTop = api.coord([p, 1])[1];
                var yBot = api.coord([p, 0])[1];
                children.push({ type: 'rect', shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } });
              }
              return { type: 'group', children: children };
            },
          };
        })(band));
      }

      return {
        grid: grids, xAxis: xAxes, yAxis: yAxes,
        tooltip: {
          trigger: 'item', confine: true, backgroundColor: '#0f172a', borderWidth: 0,
          textStyle: { color: '#fff', fontSize: 12 },
          formatter: function (params) {
            if (params.seriesType === 'scatter') {
              var parts = cats.map(function (c) { return c.name + ': ' + (c.values[params.name] || ''); });
              return params.name + (parts.length ? '<br/>' + parts.join('<br/>') : '');
            }
            return '';
          },
        },
        series: series,
      };
    }

    var chart = echarts.init(document.getElementById('sample-tree'));
    chart.setOption(buildOption(palettesFor(bands)));

    // key selector: buttons toggle which sample_info keys are shown
    var ctrl = document.getElementById('sample-tree-controls');
    if (ctrl && bands.length) {
      var shown = bands.map(function () { return true; });
      function renderControls() {
        ctrl.innerHTML = '';
        bands.forEach(function (band, i) {
          var b = document.createElement('button');
          b.type = 'button';
          b.textContent = band.name;
          b.className = 'st-key' + (shown[i] ? ' on' : '');
          b.onclick = function () {
            shown[i] = !shown[i];
            var active = bands.filter(function (_, k) { return shown[k]; });
            chart.setOption(buildOption(palettesFor(active)), true);
            renderControls();
          };
          ctrl.appendChild(b);
        });
      }
      renderControls();
    }

    window.addEventListener('resize', function () { chart.resize(); });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initSampleTree);
  } else {
    initSampleTree();
  }
})();
