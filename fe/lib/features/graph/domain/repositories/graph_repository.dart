import '../../../../core/error/result.dart';
import '../entities/graph_data.dart';

/// Contratto del repository del grafo (domain). Il bloc dipende solo da questa
/// astrazione.
abstract class GraphRepository {
  /// Carica l'intero grafo (nodi + archi).
  Future<Result<GraphData>> getGraph();
}
