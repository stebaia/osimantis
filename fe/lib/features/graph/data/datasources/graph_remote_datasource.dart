import 'package:dio/dio.dart';

import '../../domain/entities/graph_data.dart';

/// Datasource remoto del grafo: parla con `GET /graph` del backend Go, che
/// ritorna `{nodes:[...], edges:[...]}`. Propaga DioException: la mappatura a
/// Failure la fa il repository.
class GraphRemoteDataSource {
  GraphRemoteDataSource(this._dio);
  final Dio _dio;

  Future<GraphData> getGraph() async {
    final res = await _dio.get<Map<String, dynamic>>('/graph');
    final json = res.data;
    if (json == null) {
      throw DioException(
        requestOptions: res.requestOptions,
        response: res,
        type: DioExceptionType.badResponse,
        error: 'Grafo vuoto',
      );
    }
    return graphFromJson(json);
  }
}

/// Mappa il JSON di /graph in GraphData. Esposta (non privata) per poterla
/// testare direttamente con una fixture.
GraphData graphFromJson(Map<String, dynamic> json) {
  return GraphData(
    nodes: _list(json['nodes'], _nodeFromJson),
    edges: _list(json['edges'], _edgeFromJson),
  );
}

GraphNode _nodeFromJson(Map<String, dynamic> j) => GraphNode(
      id: (j['id'] as num).toInt(),
      type: (j['type'] as String?) ?? 'person',
      name: (j['name'] as String?) ?? '',
      data: (j['data'] as Map?)?.cast<String, dynamic>() ?? const {},
    );

GraphEdge _edgeFromJson(Map<String, dynamic> j) => GraphEdge(
      id: (j['id'] as num?)?.toInt() ?? 0,
      from: (j['from'] as num).toInt(),
      to: (j['to'] as num).toInt(),
      type: (j['type'] as String?) ?? '',
      weight: (j['weight'] as num?)?.toDouble() ?? 1.0,
    );

List<T> _list<T>(dynamic raw, T Function(Map<String, dynamic>) f) {
  if (raw is! List) return const [];
  return raw
      .whereType<Map>()
      .map((m) => f(m.cast<String, dynamic>()))
      .toList(growable: false);
}
