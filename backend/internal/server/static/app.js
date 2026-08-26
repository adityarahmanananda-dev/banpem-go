(function () {
  "use strict";

  // ---- Format uang (ribuan titik) pada input .money-input ----
  function digitsOnly(v) {
    return (v || "").replace(/[^0-9]/g, "");
  }
  function formatThousands(v) {
    return v.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  }
  document.querySelectorAll(".money-input").forEach(function (el) {
    var fmt = function () {
      var d = digitsOnly(el.value);
      el.value = formatThousands(d);
    };
    el.addEventListener("input", fmt);
    fmt();
  });
  // Sebelum submit, biarkan server mem-parse (money.Parse menerima "1.000.000").

  // ---- Modal Export ----
  var exportModalEl = document.getElementById("exportModal");
  if (exportModalEl) {
    var exportForm = document.getElementById("exportForm");
    var tanggalCetak = document.getElementById("exportTanggalCetak");
    var versiWrap = document.getElementById("exportVersiWrap");
    var orientasi = document.getElementById("exportOrientasi");
    var today = new Date();
    tanggalCetak.value = today.getFullYear() + "-" +
      String(today.getMonth() + 1).padStart(2, "0") + "-" +
      String(today.getDate()).padStart(2, "0");

    document.querySelectorAll("[data-export-url]").forEach(function (link) {
      link.addEventListener("click", function (e) {
        e.preventDefault();
        exportForm.querySelectorAll(".export-col-input").forEach(function (el) {
          el.remove();
        });
        // Bawa pilihan kolom tambahan halaman (mis. rekap penggunaan dana)
        // sebagai hidden input agar ikut dikirim saat export.
        var params = new URLSearchParams(window.location.search);
        ["kegiatan", "sub_kegiatan", "aktivitas", "komponen", "pajak"].forEach(function (name) {
          var v = params.get(name);
          if (v) {
            var input = document.createElement("input");
            input.type = "hidden";
            input.name = name;
            input.value = v;
            input.className = "export-col-input";
            exportForm.appendChild(input);
          }
        });
        exportForm.action = link.getAttribute("data-export-url");
        if (link.hasAttribute("data-versi")) {
          versiWrap.style.display = "";
          exportForm.versi.value = "rencana";
        } else {
          versiWrap.style.display = "none";
        }
        new bootstrap.Modal(exportModalEl).show();
      });
    });
    exportForm.addEventListener("submit", function () {
      if (versiWrap.style.display === "none") {
        exportForm.versi.disabled = true;
      }
      if (!orientasi.value) {
        orientasi.value = "landscape";
      }
    });
  }

  // ---- Sortable drag & drop untuk baris ledger / rekap belanja ----
  function initSortable(tbody) {
    var url = tbody.getAttribute("data-url");
    if (!url) return;
    new Sortable(tbody, {
      animation: 150,
      handle: "tr",
      filter: ".no-drag",
      onEnd: function () {
        var order = [];
        tbody.querySelectorAll("tr").forEach(function (tr) {
          var id = tr.getAttribute("data-id");
          if (id) order.push(Number(id));
        });
        fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ order: order })
        }).then(function (resp) {
          if (!resp.ok) alert("Gagal menyimpan urutan.");
        });
      }
    });
  }
  document.querySelectorAll("tbody[data-url]").forEach(initSortable);

  // ---- Setor pajak: total nominal tercentang ----
  function rupiahFromSen(total) {
    total = Number(total || 0);
    var neg = total < 0;
    if (neg) total = -total;
    var whole = Math.round(total / 100);
    return (neg ? "-Rp." : "Rp.") + formatThousands(String(whole));
  }
  var setorTotalEl = document.getElementById("setorTotal");
  if (setorTotalEl) {
    function updateSetorTotal() {
      var total = 0;
      document.querySelectorAll(".setor-check:checked").forEach(function (c) {
        total += Number(c.getAttribute("data-nilai") || 0);
      });
      setorTotalEl.textContent = rupiahFromSen(total);
    }
    document.querySelectorAll(".setor-check").forEach(function (c) {
      c.addEventListener("change", updateSetorTotal);
    });
    updateSetorTotal();
  }

  // ---- Setor pajak: ganti jenis pajak -> reload form dengan filter invoice ----
  var jenisPajakEl = document.getElementById("jenisPajak");
  if (jenisPajakEl) {
    jenisPajakEl.addEventListener("change", function () {
      var url = new URL(window.location.href);
      url.searchParams.set("jenis", jenisPajakEl.value);
      window.location.href = url.toString();
    });
  }

  // ---- Form tagihan: preview pajak ----
  function moneyDisp(v) {
    return rupiahFromSen(Number(v || 0));
  }
  function moneyPlain(v) {
    v = Number(v || 0);
    return formatThousands(String(Math.round(v / 100)));
  }
  function updateNetto() {
    var bruto = document.getElementById("bruto");
    var ppn = document.getElementById("pvPpn");
    var pph = document.getElementById("pvPph");
    var admin = document.getElementById("pvAdmin");
    if (!bruto || !ppn || !pph || !admin) return;
    var b = parseInt(digitsOnly(bruto.value) || "0", 10);
    var p = parseInt(digitsOnly(ppn.value) || "0", 10);
    var h = parseInt(digitsOnly(pph.value) || "0", 10);
    var a = parseInt(admin.getAttribute("data-sen") || "0", 10);
    document.getElementById("pvNetto").textContent = moneyDisp(b - p - h - a);
  }
  function hitungPajak() {
    var bruto = document.getElementById("bruto");
    if (!bruto || !window.BANTUAN_ID) return;
    var params = new URLSearchParams({
      bruto: digitsOnly(bruto.value),
      jenis_menu: document.getElementById("jenisMenu").value,
      flag_ppn: document.querySelector(".flag-ppn").value,
      flag_pph: document.querySelector(".flag-pph").value,
      bank: document.getElementById("bank").value
    });
    var kat = document.querySelector('select[name="kategori_narasumber"]');
    if (kat && kat.value) params.set("kategori_narasumber", kat.value);
    fetch("/api/bantuan/" + window.BANTUAN_ID + "/hitung-pajak?" + params.toString())
      .then(function (r) { return r.json(); })
      .then(function (res) {
        document.getElementById("pvDpp").textContent = moneyDisp(res.dpp);
        document.getElementById("pvDppNilai").textContent = moneyDisp(res.dpp_nilai_lain);
        document.getElementById("pvPpn").value = moneyPlain(res.ppn);
        document.getElementById("pvJenisPph").textContent = res.jenis_pph || "-";
        document.getElementById("pvPph").value = moneyPlain(res.pph);
        document.getElementById("pvAdmin").textContent = moneyDisp(res.biaya_admin);
        document.getElementById("pvAdmin").setAttribute("data-sen", res.biaya_admin);
        updateNetto();
      })
      .catch(function () {});
  }
  var brutoEl = document.getElementById("bruto");
  if (brutoEl && window.BANTUAN_ID) {
    var pajakTriggers = [
      "jenisMenu", "bruto", "bank"
    ].map(function (id) { return document.getElementById(id); }).filter(Boolean);
    pajakTriggers.push.apply(pajakTriggers, document.querySelectorAll(".flag-ppn, .flag-pph, select[name=kategori_narasumber]"));
    pajakTriggers.forEach(function (el) {
      el.addEventListener("change", hitungPajak);
      el.addEventListener("input", hitungPajak);
    });
    // Koreksi manual PPN/PPh ikut menghitung ulang netto.
    ["pvPpn", "pvPph"].forEach(function (id) {
      var el = document.getElementById(id);
      if (el) {
        el.addEventListener("input", updateNetto);
        el.addEventListener("change", updateNetto);
      }
    });
    if (window.IS_EDIT) {
      // Saat edit, pertahankan nilai pajak tersimpan (bisa hasil koreksi).
      updateNetto();
    } else {
      hitungPajak();
    }
  }

  // ---- Tampilkan/menyembunyikan blok pajak berdasarkan jenis pengeluaran ----
  // Jenis 4 (Transport) & 5 (Uang Harian) tanpa pajak, jadi blok No. Bupot PPh
  // hanya muncul untuk jenis 3 (Honor Peserta).
  function togglePajakMenu() {
    var jm = document.getElementById("jenisMenu");
    if (!jm) return;
    var v = Number(jm.value);
    document.getElementById("pajakMenu1").style.display = v === 1 ? "" : "none";
    document.getElementById("pajakMenu2").style.display = v === 2 ? "" : "none";
    document.getElementById("pajakMenu3").style.display = v === 3 ? "" : "none";
  }
  var jmEl = document.getElementById("jenisMenu");
  if (jmEl) {
    jmEl.addEventListener("change", togglePajakMenu);
    togglePajakMenu();
  }

  // ---- Sembunyikan No. Bupot saat pajak dipilih "Tidak ada" ----
  function toggleBupotFields() {
    var flagPPN = document.querySelector(".flag-ppn");
    var flagPPH = document.querySelector(".flag-pph");
    var bPPN = document.getElementById("bupotPPN");
    var bPPH = document.getElementById("bupotPPH");
    if (flagPPN && bPPN) bPPN.style.display = Number(flagPPN.value) === 1 ? "" : "none";
    if (flagPPH && bPPH) bPPH.style.display = Number(flagPPH.value) === 3 ? "none" : "";
  }
  var flagPPNEl = document.querySelector(".flag-ppn");
  var flagPPHEl = document.querySelector(".flag-pph");
  if (flagPPNEl && flagPPHEl) {
    flagPPNEl.addEventListener("change", toggleBupotFields);
    flagPPHEl.addEventListener("change", toggleBupotFields);
    toggleBupotFields();
  }

  // ---- Master data kegiatan: cascade kegiatan > sub > aktivitas > komponen ----
  var MASTER = window.MASTER || [];
  function masterOptions(arr, selected) {
    var opts = '<option value="">Pilih</option>';
    arr.forEach(function (m) {
      var sel = selected && Number(selected) === Number(m.ID) ? " selected" : "";
      opts += '<option value="' + m.ID + '"' + sel + ">" + m.Nama + "</option>";
    });
    return opts;
  }
  function findSub(subId) {
    for (var i = 0; i < MASTER.length; i++) {
      var subs = MASTER[i].Subs || [];
      for (var j = 0; j < subs.length; j++) {
        if (Number(subs[j].ID) === Number(subId)) return subs[j];
      }
    }
    return null;
  }
  function findAktivitas(aktId) {
    for (var i = 0; i < MASTER.length; i++) {
      var subs = MASTER[i].Subs || [];
      for (var j = 0; j < subs.length; j++) {
        var acts = subs[j].Aktivitass || [];
        for (var k = 0; k < acts.length; k++) {
          if (Number(acts[k].ID) === Number(aktId)) return acts[k];
        }
      }
    }
    return null;
  }
  function findKomponen(kid) {
    for (var i = 0; i < MASTER.length; i++) {
      var subs = MASTER[i].Subs || [];
      for (var j = 0; j < subs.length; j++) {
        var acts = subs[j].Aktivitass || [];
        for (var k = 0; k < acts.length; k++) {
          var ks = acts[k].Komponens || [];
          for (var m = 0; m < ks.length; m++) {
            if (Number(ks[m].ID) === Number(kid)) return ks[m];
          }
        }
      }
    }
    return null;
  }
  function findKegByKomponen(kid) {
    for (var i = 0; i < MASTER.length; i++) {
      var subs = MASTER[i].Subs || [];
      for (var j = 0; j < subs.length; j++) {
        var acts = subs[j].Aktivitass || [];
        for (var k = 0; k < acts.length; k++) {
          var ks = acts[k].Komponens || [];
          for (var m = 0; m < ks.length; m++) {
            if (Number(ks[m].ID) === Number(kid)) return { keg: MASTER[i], sub: subs[j], akt: acts[k] };
          }
        }
      }
    }
    return null;
  }

  function initRealisasiRow(row) {
    var kegSel = row.querySelector(".r-kegiatan");
    var subSel = row.querySelector(".r-sub");
    var aktSel = row.querySelector(".r-aktivitas");
    var komSel = row.querySelector(".r-komponen");
    if (!kegSel || !subSel || !aktSel || !komSel) return;

    var komID = komSel.getAttribute("data-komponen");
    var subID = subSel.getAttribute("data-sub");
    var aktID = aktSel.getAttribute("data-aktivitas");
    var preselect = komID ? findKegByKomponen(komID) : null;
    if (preselect) {
      subID = preselect.sub.ID;
      aktID = preselect.akt.ID;
    }

    kegSel.innerHTML = masterOptions(MASTER, preselect ? preselect.keg.ID : "");
    var kegID = preselect ? preselect.keg.ID : "";
    var selectedSubs = kegID
      ? MASTER.filter(function (m) { return Number(m.ID) === Number(kegID); })[0].Subs || []
      : [];
    subSel.innerHTML = masterOptions(selectedSubs, subID);
    subSel.disabled = !kegID;
    var selectedActs = subID ? (findSub(subID) || {}).Aktivitass || [] : [];
    aktSel.innerHTML = masterOptions(selectedActs, aktID);
    aktSel.disabled = !subID;
    var selectedKoms = aktID ? (findAktivitas(aktID) || {}).Komponens || [] : [];
    komSel.innerHTML = masterOptions(selectedKoms, komID);
    komSel.disabled = !aktID;

    kegSel.addEventListener("change", function () {
      var k = kegSel.value;
      var subs = k ? MASTER.filter(function (m) { return Number(m.ID) === Number(k); })[0].Subs || [] : [];
      subSel.innerHTML = masterOptions(subs, "");
      subSel.disabled = !k;
      aktSel.innerHTML = '<option value="">Pilih</option>';
      aktSel.disabled = !k;
      komSel.innerHTML = '<option value="">Pilih</option>';
      komSel.disabled = !k;
    });
    subSel.addEventListener("change", function () {
      var sk = subSel.value;
      var acts = sk ? (findSub(sk) || {}).Aktivitass || [] : [];
      aktSel.innerHTML = masterOptions(acts, "");
      aktSel.disabled = !sk;
      komSel.innerHTML = '<option value="">Pilih</option>';
      komSel.disabled = !sk;
    });
    aktSel.addEventListener("change", function () {
      var ak = aktSel.value;
      var koms = ak ? (findAktivitas(ak) || {}).Komponens || [] : [];
      komSel.innerHTML = masterOptions(koms, "");
      komSel.disabled = !ak;
    });
    var hapusBtn = row.querySelector(".r-hapus");
    if (hapusBtn) {
      hapusBtn.addEventListener("click", function () {
        row.remove();
      });
    }
  }
  var rowsContainer = document.getElementById("realisasiRows");
  if (rowsContainer) {
    rowsContainer.querySelectorAll(".realisasi-row").forEach(initRealisasiRow);
    document.getElementById("tambahRealisasi").addEventListener("click", function () {
      var tpl = document.createElement("div");
      tpl.className = "realisasi-row row g-2 mb-2";
      tpl.innerHTML =
        '<div class="col-12 col-md-2"><select class="form-select form-select-sm r-kegiatan"></select></div>' +
        '<div class="col-12 col-md-2"><select class="form-select form-select-sm r-sub"></select></div>' +
        '<div class="col-12 col-md-2"><select class="form-select form-select-sm r-aktivitas"></select></div>' +
        '<div class="col-12 col-md-2"><select class="form-select form-select-sm r-komponen" name="komponen_id"></select></div>' +
        '<div class="col-6 col-md-2"><input type="text" class="form-control form-control-sm money-input r-bruto" name="bruto"></div>' +
        '<div class="col-6 col-md-2 d-flex align-items-center"><button type="button" class="btn btn-outline-danger btn-sm r-hapus"><i class="bi bi-x-lg"></i></button></div>';
      rowsContainer.appendChild(tpl);
      var inp = tpl.querySelector(".money-input");
      inp.addEventListener("input", function () {
        var d = digitsOnly(inp.value);
        inp.value = formatThousands(d);
      });
      initRealisasiRow(tpl);
    });
  }

  // ---- RAB: tombol Edit menampilkan/menyembunyikan aksi edit tiap baris ----
  var toggleEditBtn = document.getElementById("toggleEdit");
  if (toggleEditBtn) {
    toggleEditBtn.addEventListener("click", function () {
      var show = toggleEditBtn.classList.toggle("active");
      document.querySelectorAll(".edit-actions").forEach(function (el) {
        el.classList.toggle("d-none", !show);
      });
      toggleEditBtn.innerHTML = show
        ? '<i class="bi bi-check-lg"></i> Selesai'
        : '<i class="bi bi-pencil"></i> Edit';
    });
  }

  // ---- Komponen (Data Kegiatan): input beberapa baris sekaligus ----
  function komponenRowHTML() {
    return '<div class="komponen-row row g-2">' +
      '<div class="col-12 col-md-6">' +
        '<input type="text" class="form-control form-control-sm" name="nama" placeholder="Nama komponen">' +
      '</div>' +
      '<div class="col-8 col-md-4">' +
        '<div class="input-group input-group-sm">' +
          '<span class="input-group-text">Rp.</span>' +
          '<input type="text" class="form-control money-input text-end" name="pagu" placeholder="0">' +
        '</div>' +
      '</div>' +
      '<div class="col-4 col-md-2 d-grid">' +
        '<button type="button" class="btn btn-outline-danger btn-sm komponen-hapus"><i class="bi bi-x-lg"></i></button>' +
      '</div>' +
    '</div>';
  }
  document.addEventListener("click", function (e) {
    if (!(e.target instanceof Element)) return;
    var addBtn = e.target.closest(".tambah-komponen");
    if (addBtn) {
      e.preventDefault();
      var container = addBtn.closest("form").querySelector(".komponen-rows");
      if (!container) return;
      container.insertAdjacentHTML("beforeend", komponenRowHTML());
      var inp = container.lastElementChild.querySelector(".money-input");
      if (inp) {
        inp.addEventListener("input", function () {
          var d = digitsOnly(inp.value);
          inp.value = formatThousands(d);
        });
      }
      return;
    }
    var delBtn = e.target.closest(".komponen-hapus");
    if (delBtn) {
      e.preventDefault();
      var container = delBtn.closest(".komponen-rows");
      if (container && container.querySelectorAll(".komponen-row").length > 1) {
        delBtn.closest(".komponen-row").remove();
      }
    }
  });
})();
