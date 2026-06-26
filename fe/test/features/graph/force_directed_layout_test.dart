import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:osimantis/features/graph/domain/entities/graph_data.dart';
import 'package:osimantis/features/graph/presentation/layout/force_directed_layout.dart';

const _size = Size(400, 800);

GraphData _sampleGraph() => const GraphData(
      nodes: [
        GraphNode(id: 1, type: 'person', name: 'A'),
        GraphNode(id: 2, type: 'person', name: 'B'),
        GraphNode(id: 3, type: 'person', name: 'C'),
        GraphNode(id: 4, type: 'place', name: 'Bar'),
      ],
      edges: [
        GraphEdge(id: 1, from: 1, to: 2, type: 'amico'),
        GraphEdge(id: 2, from: 2, to: 3, type: 'amico'),
        GraphEdge(id: 3, from: 1, to: 4, type: 'frequenta'),
      ],
    );

void main() {
  group('forceDirectedLayout', () {
    test('è deterministico con lo stesso seed', () {
      final a = forceDirectedLayout(_sampleGraph(), _size, seed: 7);
      final b = forceDirectedLayout(_sampleGraph(), _size, seed: 7);
      expect(a.keys.toSet(), b.keys.toSet());
      for (final id in a.keys) {
        expect(a[id]!.dx, closeTo(b[id]!.dx, 1e-9));
        expect(a[id]!.dy, closeTo(b[id]!.dy, 1e-9));
      }
    });

    test('produce una posizione per ogni nodo, dentro il canvas', () {
      final pos = forceDirectedLayout(_sampleGraph(), _size, seed: 1);
      expect(pos.keys.toSet(), {1, 2, 3, 4});
      for (final p in pos.values) {
        expect(p.dx, inInclusiveRange(0, _size.width));
        expect(p.dy, inInclusiveRange(0, _size.height));
      }
    });

    test('non sovrappone i nodi (distanza minima > 0)', () {
      final pos = forceDirectedLayout(_sampleGraph(), _size, seed: 3);
      final list = pos.values.toList();
      for (var i = 0; i < list.length; i++) {
        for (var j = i + 1; j < list.length; j++) {
          expect((list[i] - list[j]).distance, greaterThan(8));
        }
      }
    });

    test('grafo vuoto → mappa vuota; nodo singolo → centrato', () {
      expect(forceDirectedLayout(const GraphData(), _size), isEmpty);
      final single = forceDirectedLayout(
        const GraphData(nodes: [GraphNode(id: 9, type: 'person', name: 'Solo')]),
        _size,
      );
      expect(single[9], const Offset(200, 400));
    });
  });
}
