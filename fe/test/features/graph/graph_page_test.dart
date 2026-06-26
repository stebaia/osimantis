import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:osimantis/core/error/result.dart';
import 'package:osimantis/features/graph/domain/entities/graph_data.dart';
import 'package:osimantis/features/graph/domain/repositories/graph_repository.dart';
import 'package:osimantis/features/graph/domain/usecases/get_graph.dart';
import 'package:osimantis/features/graph/presentation/bloc/graph_bloc.dart';
import 'package:osimantis/features/graph/presentation/pages/graph_page.dart';
import 'package:osimantis/features/graph/presentation/widgets/node_chip.dart';

class _FakeGraphRepo implements GraphRepository {
  _FakeGraphRepo(this._graph);
  final GraphData _graph;
  @override
  Future<Result<GraphData>> getGraph() async => Success(_graph);
}

const _graph = GraphData(
  nodes: [
    GraphNode(id: 2, type: 'person', name: 'Erik Muratori'),
    GraphNode(id: 7, type: 'person', name: 'Stefano Baiardi'),
    GraphNode(id: 8, type: 'place', name: 'Rift'),
  ],
  edges: [
    GraphEdge(id: 4, from: 7, to: 2, type: 'amico'),
  ],
);

void main() {
  testWidgets('GraphPage rende i nodi (non resta vuota)', (tester) async {
    final bloc = GraphBloc(getGraph: GetGraph(_FakeGraphRepo(_graph)))
      ..add(const GraphLoadRequested());

    await tester.pumpWidget(MaterialApp(home: GraphPage(bloc: bloc)));
    await tester.pumpAndSettle();

    // I tre nodi devono essere effettivamente presenti e disegnati.
    expect(find.byType(NodeChip), findsNWidgets(3));
    expect(find.text('Stefano Baiardi'), findsOneWidget);
    expect(find.text('Rift'), findsOneWidget);
  });
}
