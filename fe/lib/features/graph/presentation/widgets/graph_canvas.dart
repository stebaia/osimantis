import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../domain/entities/graph_data.dart';

/// Disegna gli ARCHI del grafo come linee tra le posizioni dei nodi. I nodi
/// stessi NON sono disegnati qui: sono widget tappabili sovrapposti in uno Stack
/// dalla pagina (vedi GraphPage). Tenere gli archi nel painter e i nodi come
/// widget è la scelta che mantiene i nodi cliccabili.
class GraphEdgesPainter extends CustomPainter {
  GraphEdgesPainter({required this.edges, required this.positions});

  /// Archi da disegnare.
  final List<GraphEdge> edges;

  /// Posizione (centro) di ogni nodo, per id.
  final Map<int, Offset> positions;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = AppColors.primary.withValues(alpha: 0.35)
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;

    for (final e in edges) {
      final a = positions[e.from];
      final b = positions[e.to];
      if (a == null || b == null) continue;
      paint.strokeWidth = (e.weight.clamp(0.5, 4.0)) * 1.5;
      canvas.drawLine(a, b, paint);
    }
  }

  @override
  bool shouldRepaint(covariant GraphEdgesPainter old) =>
      old.edges != edges || old.positions != positions;
}
