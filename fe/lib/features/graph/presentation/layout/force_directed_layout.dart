import 'dart:math' as math;
import 'dart:ui';

import '../../domain/entities/graph_data.dart';

/// Forza dell'attrazione verso il centro applicata a ogni nodo a ogni passo.
/// Piccola: deve solo impedire ai nodi isolati di fuggire ai bordi, senza
/// schiacciare il grafo in un punto.
const double _gravity = 0.03;

/// Calcola le posizioni dei nodi con un layout force-directed (Fruchterman–
/// Reingold semplificato), in UNA passata one-shot: simula per [iterations] passi
/// e restituisce le posizioni finali normalizzate dentro [size]. NON è
/// un'animazione continua: si calcola al load e poi resta statico.
///
/// Modello: repulsione tra TUTTI i nodi (li allontana, evita sovrapposizioni) +
/// attrazione lungo gli ARCHI (avvicina i nodi collegati). Le posizioni iniziali
/// sono deterministiche dato [seed]: stesso grafo + stesso seed → stesso layout
/// (testabile).
///
/// Restituisce una mappa id-nodo → posizione (centro) entro [size], con un
/// margine [padding] dai bordi.
Map<int, Offset> forceDirectedLayout(
  GraphData graph,
  Size size, {
  int iterations = 300,
  int seed = 1,
  double padding = 48,
}) {
  final nodes = graph.nodes;
  if (nodes.isEmpty) return const {};
  if (nodes.length == 1) {
    return {nodes.first.id: Offset(size.width / 2, size.height / 2)};
  }

  final area = size.width * size.height;
  // Distanza "ideale" tra nodi collegati (k): cresce con l'area, cala col numero.
  // Il fattore 1.3 (>1) dà respiro: i nodi sono chip larghi ~150px, non punti,
  // quindi serve più spazio tra i centri per non farli toccare.
  final k = 1.3 * math.sqrt(area / nodes.length);

  final rnd = math.Random(seed);
  // Posizioni iniziali sparse ma deterministiche.
  final pos = <int, _V>{};
  for (final n in nodes) {
    pos[n.id] = _V(
      (rnd.nextDouble() - 0.5) * size.width,
      (rnd.nextDouble() - 0.5) * size.height,
    );
  }

  // Adiacenze (archi) verso coppie di id validi.
  final edges = graph.edges
      .where((e) => pos.containsKey(e.from) && pos.containsKey(e.to))
      .toList(growable: false);

  // Temperatura: ampiezza massima dello spostamento, decresce ogni iterazione.
  var temp = math.min(size.width, size.height) / 4;
  final cooling = temp / (iterations + 1);

  final disp = <int, _V>{for (final n in nodes) n.id: _V(0, 0)};

  for (var it = 0; it < iterations; it++) {
    for (final v in disp.values) {
      v
        ..x = 0
        ..y = 0;
    }

    // Repulsione tra ogni coppia di nodi.
    for (var i = 0; i < nodes.length; i++) {
      final vi = pos[nodes[i].id]!;
      for (var j = i + 1; j < nodes.length; j++) {
        final vj = pos[nodes[j].id]!;
        var dx = vi.x - vj.x;
        var dy = vi.y - vj.y;
        var dist = math.sqrt(dx * dx + dy * dy);
        if (dist < 0.01) {
          // Nodi sovrapposti: spingili in una direzione deterministica.
          dx = 0.01 * (i - j);
          dy = 0.01;
          dist = 0.01;
        }
        final force = (k * k) / dist;
        final fx = (dx / dist) * force;
        final fy = (dy / dist) * force;
        disp[nodes[i].id]!
          ..x += fx
          ..y += fy;
        disp[nodes[j].id]!
          ..x -= fx
          ..y -= fy;
      }
    }

    // Attrazione lungo gli archi.
    for (final e in edges) {
      final a = pos[e.from]!;
      final b = pos[e.to]!;
      var dx = a.x - b.x;
      var dy = a.y - b.y;
      var dist = math.sqrt(dx * dx + dy * dy);
      if (dist < 0.01) dist = 0.01;
      final force = (dist * dist) / k;
      final fx = (dx / dist) * force;
      final fy = (dy / dist) * force;
      disp[e.from]!
        ..x -= fx
        ..y -= fy;
      disp[e.to]!
        ..x += fx
        ..y += fy;
    }

    // Gravità verso il centro: senza questa, i nodi SENZA archi (isolati) sono
    // soggetti solo alla repulsione e schizzano ai bordi. Una leggera attrazione
    // verso (0,0) li tiene raccolti e compatta l'intero grafo.
    for (final n in nodes) {
      final p = pos[n.id]!;
      disp[n.id]!
        ..x -= p.x * _gravity
        ..y -= p.y * _gravity;
    }

    // Applica gli spostamenti, limitati dalla temperatura.
    for (final n in nodes) {
      final d = disp[n.id]!;
      final len = math.sqrt(d.x * d.x + d.y * d.y);
      if (len < 0.001) continue;
      final limited = math.min(len, temp);
      final p = pos[n.id]!;
      p
        ..x += (d.x / len) * limited
        ..y += (d.y / len) * limited;
    }

    temp = math.max(temp - cooling, 0.0);
  }

  final placed = _normalize(pos, size, padding);
  // Le forze trattano i nodi come PUNTI, ma i chip sono larghi (~160px): i nodi
  // connessi collassavano in un mucchio sovrapposto. Qui, in pixel reali del
  // canvas, separiamo a forza ogni coppia troppo vicina fino a [_minSeparation],
  // così i chip non si accavallano più. Poche passate bastano a sciogliere i
  // grovigli senza spingere nessuno fuori dai bordi.
  return _separate(placed, size, padding);
}

/// Distanza minima imposta tra i CENTRI dei nodi, in pixel. Tarata sulla
/// larghezza tipica dei chip così non si sovrappongono.
const double _minSeparation = 120;

/// Spinge via le coppie di nodi più vicine di [_minSeparation], iterando qualche
/// passata, e ri-clampa dentro i bordi (con [padding]).
Map<int, Offset> _separate(Map<int, Offset> input, Size size, double padding) {
  final ids = input.keys.toList(growable: false);
  final p = Map<int, Offset>.from(input);

  Offset clampIn(Offset o) => Offset(
        o.dx.clamp(padding, size.width - padding),
        o.dy.clamp(padding, size.height - padding),
      );

  for (var pass = 0; pass < 30; pass++) {
    var moved = false;
    for (var i = 0; i < ids.length; i++) {
      for (var j = i + 1; j < ids.length; j++) {
        final a = p[ids[i]]!;
        final b = p[ids[j]]!;
        var dx = a.dx - b.dx;
        var dy = a.dy - b.dy;
        var dist = math.sqrt(dx * dx + dy * dy);
        if (dist >= _minSeparation) continue;
        if (dist < 0.01) {
          // Esattamente sovrapposti: separazione deterministica.
          dx = (i - j).toDouble();
          dy = 1;
          dist = math.sqrt(dx * dx + dy * dy);
        }
        final push = (_minSeparation - dist) / 2;
        final ux = dx / dist;
        final uy = dy / dist;
        // Clampa SUBITO dentro i bordi, così separazione e bordi convergono
        // insieme invece di combattersi (un clamp finale annullerebbe la
        // separazione vicino ai bordi).
        p[ids[i]] = clampIn(a.translate(ux * push, uy * push));
        p[ids[j]] = clampIn(b.translate(-ux * push, -uy * push));
        moved = true;
      }
    }
    if (!moved) break;
  }

  return p;
}

/// Riscala le posizioni grezze dentro [size] con un margine [padding].
Map<int, Offset> _normalize(Map<int, _V> pos, Size size, double padding) {
  var minX = double.infinity, minY = double.infinity;
  var maxX = -double.infinity, maxY = -double.infinity;
  for (final v in pos.values) {
    minX = math.min(minX, v.x);
    minY = math.min(minY, v.y);
    maxX = math.max(maxX, v.x);
    maxY = math.max(maxY, v.y);
  }
  final spanX = (maxX - minX).abs() < 1e-6 ? 1.0 : (maxX - minX);
  final spanY = (maxY - minY).abs() < 1e-6 ? 1.0 : (maxY - minY);
  final w = size.width - 2 * padding;
  final h = size.height - 2 * padding;

  return {
    for (final entry in pos.entries)
      entry.key: Offset(
        padding + ((entry.value.x - minX) / spanX) * w,
        padding + ((entry.value.y - minY) / spanY) * h,
      ),
  };
}

/// Vettore mutabile interno (più economico di creare tanti Offset immutabili).
class _V {
  _V(this.x, this.y);
  double x;
  double y;
}
