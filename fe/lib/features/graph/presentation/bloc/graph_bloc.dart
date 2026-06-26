// ignore_for_file: prefer_initializing_formals
import 'package:equatable/equatable.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/result.dart';
import '../../domain/entities/graph_data.dart';
import '../../domain/usecases/get_graph.dart';

part 'graph_event.dart';
part 'graph_state.dart';

/// Carica il grafo (nodi + archi) da GET /graph e lo espone alla GraphPage.
/// Il calcolo delle posizioni dei nodi (layout) è fatto a parte (vedi lo step
/// force-directed) e qui non è ancora presente: il render iniziale usa posizioni
/// di default calcolate nella pagina.
class GraphBloc extends Bloc<GraphEvent, GraphState> {
  GraphBloc({required GetGraph getGraph})
      : _getGraph = getGraph,
        super(const GraphState()) {
    on<GraphLoadRequested>(_onLoadRequested);
  }

  final GetGraph _getGraph;

  Future<void> _onLoadRequested(
    GraphLoadRequested event,
    Emitter<GraphState> emit,
  ) async {
    emit(state.copyWith(status: GraphStatus.loading, clearError: true));
    final result = await _getGraph();
    switch (result) {
      case Success(:final data):
        emit(state.copyWith(status: GraphStatus.loaded, graph: data));
      case Error(:final failure):
        emit(state.copyWith(
          status: GraphStatus.error,
          errorMessage: failure.message,
        ));
    }
  }
}
