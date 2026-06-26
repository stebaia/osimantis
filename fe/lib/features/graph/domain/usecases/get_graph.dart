import '../../../../core/error/result.dart';
import '../entities/graph_data.dart';
import '../repositories/graph_repository.dart';

/// Usecase: carica l'intero grafo delle relazioni. Sottile, ma tiene il bloc
/// disaccoppiato dal repository.
class GetGraph {
  const GetGraph(this._repository);
  final GraphRepository _repository;

  Future<Result<GraphData>> call() => _repository.getGraph();
}
