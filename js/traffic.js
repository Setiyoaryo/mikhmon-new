/* Shared by dashboard and Traffic Monitor. jQuery encodes query parameters
 * and decodes JSON; missing samples never get plotted as zero. */
function mikhmonRequestTraffic(chart, session, iface) {
    if (!iface || chart.mikhmonTrafficPending) {
        return;
    }
    chart.mikhmonTrafficPending = true;
    $.ajax({
        url: './traffic/traffic.php',
        data: { session: session, iface: iface },
        dataType: 'json',
        timeout: 20000,
        success: function (sample) {
            var tx = sample && sample[0] && sample[0].data && sample[0].data[0];
            var rx = sample && sample[1] && sample[1].data && sample[1].data[0];
            if (typeof tx !== 'number' || typeof rx !== 'number' ||
                !isFinite(tx) || !isFinite(rx) || tx < 0 || rx < 0) {
                chart.setSubtitle({ text: 'Traffic unavailable' });
                return;
            }
            var now = new Date().getTime();
            var shift = chart.series[0].data.length >= 20;
            chart.series[0].addPoint([now, tx], false, shift);
            chart.series[1].addPoint([now, rx], false, shift);
            chart.setSubtitle({ text: '' });
            chart.redraw();
        },
        error: function () {
            chart.setSubtitle({ text: 'Traffic unavailable' });
        },
        complete: function () {
            chart.mikhmonTrafficPending = false;
        }
    });
}
