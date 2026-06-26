part of 'graph_bloc.dart';

/// Stato della GraphPage.
/// - [status] guida loading/errore/contenuto.
/// - [graph] sono i nodi e gli archi caricati da /graph.
class GraphState extends Equatable {
  const GraphState({
    this.status = GraphStatus.initial,
    this.graph = const GraphData(),
    this.errorMessage,
  });

  final GraphStatus status;
  final GraphData graph;
  final String? errorMessage;

  GraphState copyWith({
    GraphStatus? status,
    GraphData? graph,
    String? errorMessage,
    bool clearError = false,
  }) {
    return GraphState(
      status: status ?? this.status,
      graph: graph ?? this.graph,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }

  @override
  List<Object?> get props => [status, graph, errorMessage];
}

enum GraphStatus { initial, loading, loaded, error }
