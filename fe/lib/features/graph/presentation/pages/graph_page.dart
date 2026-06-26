import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/theme/app_colors.dart';
import '../../domain/entities/graph_data.dart';
import '../bloc/graph_bloc.dart';
import '../widgets/graph_canvas.dart';
import '../widgets/node_chip.dart';

/// Spazio del grafo: tutte le persone e i luoghi come nodi collegati dalle loro
/// relazioni. Carica /graph e li dispone su un canvas.
///
/// Step 2 (render statico): posizioni iniziali a cerchio, archi disegnati con
/// CustomPaint, nodi come chip. Pan/zoom e tap arrivano negli step successivi.
class GraphPage extends StatelessWidget {
  const GraphPage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => GraphBloc(getGraph: sl())..add(const GraphLoadRequested()),
      child: const _GraphView(),
    );
  }
}

class _GraphView extends StatelessWidget {
  const _GraphView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        backgroundColor: AppColors.background,
        elevation: 0,
        title: const Text('Spazio', style: TextStyle(color: AppColors.textPrimary)),
        iconTheme: const IconThemeData(color: AppColors.textPrimary),
      ),
      body: BlocBuilder<GraphBloc, GraphState>(
        builder: (context, state) {
          switch (state.status) {
            case GraphStatus.initial:
            case GraphStatus.loading:
              return const Center(child: CircularProgressIndicator());
            case GraphStatus.error:
              return Center(
                child: Text(
                  state.errorMessage ?? 'Errore nel caricamento',
                  style: const TextStyle(color: AppColors.textSecondary),
                ),
              );
            case GraphStatus.loaded:
              if (state.graph.nodes.isEmpty) {
                return const Center(
                  child: Text(
                    'Ancora nessuna relazione da mostrare',
                    style: TextStyle(color: AppColors.textSecondary),
                  ),
                );
              }
              return _GraphCanvas(graph: state.graph);
          }
        },
      ),
    );
  }
}

class _GraphCanvas extends StatelessWidget {
  const _GraphCanvas({required this.graph});
  final GraphData graph;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final size = Size(constraints.maxWidth, constraints.maxHeight);
        final positions = _circleLayout(graph.nodes, size);

        return Stack(
          children: [
            // Archi (linee) sotto i nodi.
            Positioned.fill(
              child: CustomPaint(
                painter: GraphEdgesPainter(
                  edges: graph.edges,
                  positions: positions,
                ),
              ),
            ),
            // Nodi: chip centrati sulla loro posizione.
            for (final node in graph.nodes)
              if (positions[node.id] case final p?)
                Positioned(
                  left: p.dx,
                  top: p.dy,
                  child: FractionalTranslation(
                    translation: const Offset(-0.5, -0.5),
                    child: NodeChip(node: node, onTap: () {}),
                  ),
                ),
          ],
        );
      },
    );
  }
}

/// Layout iniziale: nodi distribuiti su un cerchio attorno al centro. Provvisorio
/// — sostituito dal layout force-directed nello step successivo.
Map<int, Offset> _circleLayout(List<GraphNode> nodes, Size size) {
  final center = Offset(size.width / 2, size.height / 2);
  final radius = math.min(size.width, size.height) * 0.36;
  final positions = <int, Offset>{};
  for (var i = 0; i < nodes.length; i++) {
    final angle = (i / nodes.length) * 2 * math.pi;
    positions[nodes[i].id] = center +
        Offset(radius * math.cos(angle), radius * math.sin(angle));
  }
  return positions;
}
