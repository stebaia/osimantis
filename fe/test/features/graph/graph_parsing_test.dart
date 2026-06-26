import 'package:flutter_test/flutter_test.dart';
import 'package:osimantis/features/graph/data/datasources/graph_remote_datasource.dart';

/// Fixture: forma reale della risposta di GET /graph (ridotta ma fedele alla
/// struttura osservata in produzione).
const _graphJson = {
  'nodes': [
    {
      'id': 2,
      'type': 'person',
      'name': 'Erik Muratori',
      'data': {'relazione_utente': 'migliore amico'},
    },
    {
      'id': 7,
      'type': 'person',
      'name': 'Stefano Baiardi',
      'data': {'is_user': true},
    },
    {
      'id': 8,
      'type': 'place',
      'name': 'Rift',
      'data': {'address': 'San Mauro Pascoli', 'type': 'bar/pub'},
    },
  ],
  'edges': [
    {'id': 4, 'from': 7, 'to': 2, 'type': 'migliore amico', 'weight': 1},
  ],
};

void main() {
  group('graphFromJson', () {
    test('parsa nodi e archi dal JSON reale di /graph', () {
      final graph = graphFromJson(Map<String, dynamic>.from(_graphJson));

      expect(graph.nodes, hasLength(3));
      expect(graph.edges, hasLength(1));

      final erik = graph.nodes.firstWhere((n) => n.id == 2);
      expect(erik.name, 'Erik Muratori');
      expect(erik.isPerson, isTrue);
      expect(erik.isUser, isFalse);

      final stefano = graph.nodes.firstWhere((n) => n.id == 7);
      expect(stefano.isUser, isTrue);

      final rift = graph.nodes.firstWhere((n) => n.id == 8);
      expect(rift.isPlace, isTrue);
      expect(rift.data['type'], 'bar/pub');

      final edge = graph.edges.single;
      expect(edge.from, 7);
      expect(edge.to, 2);
      expect(edge.type, 'migliore amico');
      expect(edge.weight, 1.0);
    });

    test('campi mancanti producono default sicuri', () {
      final graph = graphFromJson({
        'nodes': [
          {'id': 1, 'name': 'X'}, // niente type/data
        ],
        // niente edges
      });
      expect(graph.nodes.single.type, 'person');
      expect(graph.nodes.single.data, isEmpty);
      expect(graph.edges, isEmpty);
    });
  });
}
