import 'package:equatable/equatable.dart';

/// L'intero grafo delle relazioni: nodi (persone e luoghi) e archi (relazioni).
/// Rispecchia `GET /graph` del backend Go. Entità di dominio: nessuna dipendenza
/// da Dio/JSON.
class GraphData extends Equatable {
  const GraphData({this.nodes = const [], this.edges = const []});

  final List<GraphNode> nodes;
  final List<GraphEdge> edges;

  @override
  List<Object?> get props => [nodes, edges];
}

/// Un nodo del grafo: una persona o un luogo.
class GraphNode extends Equatable {
  const GraphNode({
    required this.id,
    required this.type,
    required this.name,
    this.data = const {},
  });

  final int id;

  /// "person" | "place".
  final String type;
  final String name;

  /// Campi liberi (lavoro, città, tipo luogo, is_user, ...).
  final Map<String, dynamic> data;

  bool get isPerson => type == 'person';
  bool get isPlace => type == 'place';

  /// Il nodo che rappresenta l'utente (data.is_user == true).
  bool get isUser => data['is_user'] == true;

  @override
  List<Object?> get props => [id, type, name, data];
}

/// Un arco del grafo: una relazione diretta tra due nodi.
class GraphEdge extends Equatable {
  const GraphEdge({
    required this.id,
    required this.from,
    required this.to,
    required this.type,
    this.weight = 1.0,
  });

  final int id;

  /// Id del nodo di partenza.
  final int from;

  /// Id del nodo di arrivo.
  final int to;

  /// Etichetta della relazione (es. 'amico', 'frequenta', 'ragazza_di').
  final String type;
  final double weight;

  @override
  List<Object?> get props => [id, from, to, type, weight];
}
